// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestMarkGloballyVisibleFeedsAsReadWithCutoff(t *testing.T) {
	f := newMarkAsReadFixture(t)
	cutoff := time.Now().UTC().Truncate(time.Microsecond).AddDate(0, 0, -30)

	user := f.user()
	other := f.user()
	visible := f.feed(user, f.category(user, false), false)
	hiddenFeed := f.feed(user, f.category(user, false), true)
	hiddenCategoryFeed := f.feed(user, f.category(user, true), false)
	otherFeed := f.feed(other, f.category(other, false), false)

	old := f.entries(20, user, visible, cutoff.Add(-time.Hour))
	recent := f.entries(2, user, visible, cutoff.Add(time.Hour))
	boundary := f.entry(user, visible, cutoff, model.EntryStatusUnread, false)
	hiddenOld := append(f.entries(2, user, hiddenFeed, cutoff.Add(-time.Hour)), f.entries(2, user, hiddenCategoryFeed, cutoff.Add(-time.Hour))...)
	f.entries(15, other, otherFeed, cutoff.Add(-time.Hour))

	count, err := f.store.MarkGloballyVisibleFeedsAsRead(user, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(len(old)) {
		t.Fatalf("changed count = %d, want %d", count, len(old))
	}
	for _, id := range append(append(recent, boundary), hiddenOld...) {
		if s, _ := f.status(id); s != model.EntryStatusUnread {
			t.Errorf("entry %d status = %q, want unread", id, s)
		}
	}
	if got := f.unreadCount(other); got != 15 {
		t.Errorf("other user's unread = %d, want 15", got)
	}
}

// AC-10: the zero cutoff keeps today's date-unconstrained behaviour, forward-dated entries included.
func TestMarkGloballyVisibleFeedsAsReadZeroCutoffIsUnchanged(t *testing.T) {
	f := newMarkAsReadFixture(t)
	user := f.user()
	visible := f.feed(user, f.category(user, false), false)
	hidden := f.feed(user, f.category(user, false), true)

	f.entries(3, user, visible, time.Now().AddDate(0, 0, -400))
	f.entries(1, user, visible, time.Now().Add(72*time.Hour))
	hiddenEntries := f.entries(2, user, hidden, time.Now().AddDate(0, 0, -400))

	count, err := f.store.MarkGloballyVisibleFeedsAsRead(user, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("changed count = %d, want 4", count)
	}
	for _, id := range hiddenEntries {
		if s, _ := f.status(id); s != model.EntryStatusUnread {
			t.Errorf("hidden entry %d status = %q, want unread", id, s)
		}
	}
}
