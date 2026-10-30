// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestMarkFeedAsReadReturnsScopedCount(t *testing.T) {
	f := newMarkAsReadFixture(t)
	cutoff := time.Now().UTC().Truncate(time.Microsecond).AddDate(0, 0, -7)

	user := f.user()
	other := f.user()
	category := f.category(user, false)
	feed := f.feed(user, category, false)
	sibling := f.feed(user, category, false)
	otherFeed := f.feed(other, f.category(other, false), false)

	old := f.entries(4, user, feed, cutoff.Add(-time.Minute))
	boundary := f.entry(user, feed, cutoff, model.EntryStatusUnread, false)
	siblingOld := f.entries(2, user, sibling, cutoff.Add(-time.Minute))
	f.entries(2, other, otherFeed, cutoff.Add(-time.Minute))

	count, err := f.store.MarkFeedAsRead(user, feed, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(len(old)) {
		t.Fatalf("changed count = %d, want %d", count, len(old))
	}
	if s, _ := f.status(boundary); s != model.EntryStatusUnread {
		t.Errorf("boundary entry status = %q, want unread", s)
	}
	for _, id := range siblingOld {
		if s, _ := f.status(id); s != model.EntryStatusUnread {
			t.Errorf("sibling feed entry %d status = %q, want unread", id, s)
		}
	}
	if got := f.unreadCount(other); got != 2 {
		t.Errorf("other user's unread = %d, want 2", got)
	}

	// Another user cannot mark this feed.
	if count, _ := f.store.MarkFeedAsRead(other, sibling, time.Now()); count != 0 {
		t.Errorf("cross-user mark changed %d, want 0", count)
	}
}

func TestMarkCategoryAsReadReturnsScopedCount(t *testing.T) {
	f := newMarkAsReadFixture(t)
	cutoff := time.Now().UTC().Truncate(time.Microsecond).AddDate(0, 0, -7)

	user := f.user()
	other := f.user()
	category := f.category(user, false)
	otherCategory := f.category(user, false)
	feedA := f.feed(user, category, false)
	feedB := f.feed(user, category, false)
	outside := f.feed(user, otherCategory, false)

	f.entries(3, user, feedA, cutoff.Add(-time.Minute))
	f.entries(2, user, feedB, cutoff.Add(-time.Minute))
	recent := f.entry(user, feedA, cutoff.Add(time.Minute), model.EntryStatusUnread, false)
	outsideOld := f.entries(4, user, outside, cutoff.Add(-time.Minute))

	count, err := f.store.MarkCategoryAsRead(user, category, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("changed count = %d, want 5", count)
	}
	if s, _ := f.status(recent); s != model.EntryStatusUnread {
		t.Errorf("recent entry status = %q, want unread", s)
	}
	for _, id := range outsideOld {
		if s, _ := f.status(id); s != model.EntryStatusUnread {
			t.Errorf("entry %d outside the category = %q, want unread", id, s)
		}
	}

	// Another user cannot mark this user's category.
	if count, _ := f.store.MarkCategoryAsRead(other, otherCategory, time.Now()); count != 0 {
		t.Errorf("cross-user mark changed %d, want 0", count)
	}
}
