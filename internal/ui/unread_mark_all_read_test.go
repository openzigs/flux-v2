// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMarkAllAsReadOlderThanDays(t *testing.T) {
	f := newUIFixture(t)
	user := f.user()
	other := f.user()
	visible := f.feed(user, f.category(user, false), false)
	hidden := f.feed(user, f.category(user, false), true)
	f.entries(20, user, visible, time.Now().AddDate(0, 0, -45))
	f.entries(3, user, visible, time.Now().AddDate(0, 0, -2))
	f.entries(4, user, hidden, time.Now().AddDate(0, 0, -45))
	f.entries(15, other, f.feed(other, f.category(other, false), false), time.Now().AddDate(0, 0, -45))

	if w := f.post(f.h.markAllAsRead, user, "/mark-all-as-read?older_than_days=abc", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid value: status %d, want 400", w.Code)
	}

	w := f.post(f.h.markAllAsRead, user, "/mark-all-as-read?older_than_days=30", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"changed_count":20`) {
		t.Fatalf("got %d %s, want 200 with changed_count 20", w.Code, w.Body.String())
	}
	// The 3 recent visible entries and the 4 hidden ones stay unread; the unread view reflects it (AC-9).
	if got := f.unread(user); got != 7 {
		t.Fatalf("unread = %d, want 7", got)
	}
	if got := f.unread(other); got != 15 {
		t.Fatalf("other user's unread = %d, want 15", got)
	}
}

func TestMarkAllAsReadWithoutAgeIsUnchanged(t *testing.T) {
	f := newUIFixture(t)
	user := f.user()
	visible := f.feed(user, f.category(user, false), false)
	hidden := f.feed(user, f.category(user, false), true)
	f.entries(2, user, visible, time.Now().AddDate(0, 0, -45))
	f.entries(1, user, visible, time.Now().Add(48*time.Hour))
	f.entries(2, user, hidden, time.Now().AddDate(0, 0, -45))

	w := f.post(f.h.markAllAsRead, user, "/mark-all-as-read", nil)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `"OK"` {
		t.Fatalf("got %d %s, want 200 \"OK\"", w.Code, w.Body.String())
	}
	if got := f.unread(user); got != 2 {
		t.Fatalf("unread = %d, want 2 (hidden entries only)", got)
	}
}
