// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestFeedCategoryMarkAsReadOlderThanDays(t *testing.T) {
	f := newUIFixture(t)
	user := f.user()
	category := f.category(user, false)
	feed := f.feed(user, category, false)
	sibling := f.feed(user, category, false)
	ids := map[string]string{"feedID": strconv.FormatInt(feed, 10), "categoryID": strconv.FormatInt(category, 10)}

	f.entries(5, user, feed, time.Now().AddDate(0, 0, -45))
	f.entries(2, user, feed, time.Now().AddDate(0, 0, -2))
	f.entries(3, user, sibling, time.Now().AddDate(0, 0, -45))
	f.entries(1, user, sibling, time.Now().AddDate(0, 0, -2))

	for name, fn := range map[string]http.HandlerFunc{
		"feed":          f.h.markFeedAsRead,
		"category":      f.h.markCategoryAsRead,
		"category-feed": f.h.markCategoryFeedAsRead,
	} {
		if w := f.post(fn, user, "/x?older_than_days=0", ids); w.Code != http.StatusBadRequest {
			t.Errorf("%s: invalid value: status %d, want 400", name, w.Code)
		}
	}

	// Feed: only the feed's 5 old entries.
	if w := f.post(f.h.markFeedAsRead, user, "/x?older_than_days=30", ids); w.Code/100 != 3 {
		t.Fatalf("feed: status %d, want a redirect", w.Code)
	}
	if got := f.unread(user); got != 6 {
		t.Fatalf("after feed: unread = %d, want 6", got)
	}

	// Category: the sibling's 3 old entries too, never the recent ones.
	if w := f.post(f.h.markCategoryAsRead, user, "/x?older_than_days=30", ids); w.Code/100 != 3 {
		t.Fatalf("category: status %d, want a redirect", w.Code)
	}
	if got := f.unread(user); got != 3 {
		t.Fatalf("after category: unread = %d, want 3", got)
	}

	// Category feed without an age keeps today's behaviour: the whole feed.
	if w := f.post(f.h.markCategoryFeedAsRead, user, "/x", ids); w.Code/100 != 3 {
		t.Fatalf("category-feed: status %d, want a redirect", w.Code)
	}
	if got := f.unread(user); got != 1 {
		t.Fatalf("after category-feed: unread = %d, want 1", got)
	}
}
