// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

func TestMarkAllAsReadBeforeDate(t *testing.T) {
	f := newMarkAsReadFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	cutoff := now.AddDate(0, 0, -30)

	user := f.user()
	other := f.user()
	feed := f.feed(user, f.category(user, false), false)
	otherFeed := f.feed(other, f.category(other, false), false)

	old := f.entries(12, user, feed, cutoff.Add(-time.Hour))                // AC-1
	recent := f.entries(3, user, feed, cutoff.Add(time.Hour))               // AC-1
	boundary := f.entry(user, feed, cutoff, model.EntryStatusUnread, false) // AC-2: exactly N days old stays unread
	starredOld := f.entry(user, feed, cutoff.Add(-time.Hour), model.EntryStatusUnread, true)
	alreadyRead := f.entry(user, feed, cutoff.Add(-time.Hour), model.EntryStatusRead, false)
	otherOld := f.entries(5, other, otherFeed, cutoff.Add(-time.Hour)) // AC-11

	count, err := f.store.MarkAllAsReadBeforeDate(user, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(len(old)+1) { // 12 old + the starred old one
		t.Fatalf("changed count = %d, want %d", count, len(old)+1) // AC-4
	}

	for _, id := range old {
		if s, _ := f.status(id); s != model.EntryStatusRead {
			t.Errorf("old entry %d status = %q, want read", id, s)
		}
	}
	for _, id := range append(recent, boundary) {
		if s, _ := f.status(id); s != model.EntryStatusUnread {
			t.Errorf("entry %d status = %q, want unread", id, s)
		}
	}
	if s, starred := f.status(starredOld); s != model.EntryStatusRead || !starred { // AC-3
		t.Errorf("starred entry = (%q, %v), want (read, true)", s, starred)
	}
	if s, _ := f.status(alreadyRead); s != model.EntryStatusRead {
		t.Errorf("already-read entry status = %q", s)
	}
	for _, id := range otherOld {
		if s, _ := f.status(id); s != model.EntryStatusUnread {
			t.Errorf("other user's entry %d status = %q, want unread", id, s)
		}
	}

	// AC-5: idempotent.
	count, err = f.store.MarkAllAsReadBeforeDate(user, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("second run changed %d, want 0", count)
	}
}
