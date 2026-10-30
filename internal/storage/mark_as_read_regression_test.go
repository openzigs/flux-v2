// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"testing"
	"time"
)

// AC-10: the untargeted actions used by REST (no older_than_days), Fever and the UI keep
// marking everything, including forward-dated entries.
func TestMarkAllAsReadIsUnchanged(t *testing.T) {
	f := newMarkAsReadFixture(t)
	user := f.user()
	feed := f.feed(user, f.category(user, false), false)
	f.entries(2, user, feed, time.Now().AddDate(0, 0, -40))
	f.entries(1, user, feed, time.Now().Add(48*time.Hour))

	if err := f.store.MarkAllAsRead(user); err != nil {
		t.Fatal(err)
	}
	if got := f.unreadCount(user); got != 0 {
		t.Fatalf("unread = %d, want 0", got)
	}
}
