// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

// Regression tests for the pre-existing mark-as-read writers, which the REST
// API (markUserAsReadHandler, markFeedAsReadHandler, markCategoryAsReadHandler),
// Fever (handleWriteFeeds, handleWriteGroups), Google Reader and the web UI
// (markAllAsRead) call. Their behaviour must not change with the new action.

func setFeedHiddenGlobally(t *testing.T, s *Storage, feedID int64) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE feeds SET hide_globally=true WHERE id=$1`, feedID); err != nil {
		t.Fatal(err)
	}
}

func TestRegressionMarkAllAsReadIgnoresDates(t *testing.T) {
	s := newTestStorage(t)
	f := newTestFixture(t, s)
	old := insertEntry(t, s, f, time.Now().Add(-90*24*time.Hour), model.EntryStatusUnread)
	recent := insertEntry(t, s, f, time.Now(), model.EntryStatusUnread)

	if err := s.MarkAllAsRead(f.userID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{old, recent} {
		if got, _ := entryStatusAndChangedAt(t, s, id); got != model.EntryStatusRead {
			t.Errorf("entry %d: expected read, got %q", id, got)
		}
	}
}

func TestRegressionMarkGloballyVisibleFeedsAsReadSkipsHiddenFeeds(t *testing.T) {
	s := newTestStorage(t)
	visible := newTestFixture(t, s)
	visibleEntry := insertEntry(t, s, visible, time.Now(), model.EntryStatusUnread)

	// A second feed for the same user, hidden from the global unread view.
	category, err := s.FirstCategory(visible.userID)
	if err != nil {
		t.Fatal(err)
	}
	hiddenFeed := &model.Feed{UserID: visible.userID, Category: category, FeedURL: "https://example.org/hidden-" + randomSuffix(t), SiteURL: "https://example.org/", Title: "Hidden"}
	if err := s.CreateFeed(hiddenFeed); err != nil {
		t.Fatal(err)
	}
	setFeedHiddenGlobally(t, s, hiddenFeed.ID)
	hiddenEntry := insertEntry(t, s, testFixture{userID: visible.userID, feedID: hiddenFeed.ID}, time.Now(), model.EntryStatusUnread)

	if err := s.MarkGloballyVisibleFeedsAsRead(visible.userID); err != nil {
		t.Fatal(err)
	}
	if got, _ := entryStatusAndChangedAt(t, s, visibleEntry); got != model.EntryStatusRead {
		t.Errorf("visible entry: expected read, got %q", got)
	}
	if got, _ := entryStatusAndChangedAt(t, s, hiddenEntry); got != model.EntryStatusUnread {
		t.Errorf("hidden-feed entry: expected unread, got %q", got)
	}
}

func TestRegressionMarkFeedAndCategoryAsReadKeepAbsoluteCutoff(t *testing.T) {
	s := newTestStorage(t)
	f := newTestFixture(t, s)
	cutoff := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	feedOld := insertEntry(t, s, f, cutoff.Add(-time.Hour), model.EntryStatusUnread)
	feedExact := insertEntry(t, s, f, cutoff, model.EntryStatusUnread)
	if err := s.MarkFeedAsRead(f.userID, f.feedID, cutoff); err != nil {
		t.Fatal(err)
	}
	if got, _ := entryStatusAndChangedAt(t, s, feedOld); got != model.EntryStatusRead {
		t.Errorf("feed: older entry expected read, got %q", got)
	}
	if got, _ := entryStatusAndChangedAt(t, s, feedExact); got != model.EntryStatusUnread {
		t.Errorf("feed: entry at the cutoff expected unread, got %q", got)
	}

	category, err := s.FirstCategory(f.userID)
	if err != nil {
		t.Fatal(err)
	}
	catOld := insertEntry(t, s, f, cutoff.Add(-time.Hour), model.EntryStatusUnread)
	catNew := insertEntry(t, s, f, cutoff.Add(time.Hour), model.EntryStatusUnread)
	if err := s.MarkCategoryAsRead(f.userID, category.ID, cutoff); err != nil {
		t.Fatal(err)
	}
	if got, _ := entryStatusAndChangedAt(t, s, catOld); got != model.EntryStatusRead {
		t.Errorf("category: older entry expected read, got %q", got)
	}
	if got, _ := entryStatusAndChangedAt(t, s, catNew); got != model.EntryStatusUnread {
		t.Errorf("category: newer entry expected unread, got %q", got)
	}
}
