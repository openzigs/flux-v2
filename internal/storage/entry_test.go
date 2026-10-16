// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"miniflux.app/v2/internal/database"
	"miniflux.app/v2/internal/model"
)

func TestTruncateStringForTSVectorField(t *testing.T) {
	const megabyte = 1024 * 1024

	// Test case 1: Short Chinese text should not be truncated
	shortText := "这是一个简短的中文测试文本"
	result := truncateStringForTSVectorField(shortText, megabyte)
	if result != shortText {
		t.Errorf("Short text should not be truncated, got %s", result)
	}

	// Test case 2: Long Chinese text should be truncated to stay under 1MB
	// Generate a long Chinese string that would exceed 1MB
	chineseChar := "汉"
	longText := strings.Repeat(chineseChar, megabyte/len(chineseChar)+1000) // Ensure it exceeds 1MB

	result = truncateStringForTSVectorField(longText, megabyte)

	// Verify the result is under 1MB
	if len(result) >= megabyte {
		t.Errorf("Truncated text should be under 1MB, got %d bytes", len(result))
	}

	// Verify the result is still valid UTF-8 and doesn't cut in the middle of a character
	if !strings.HasPrefix(longText, result) {
		t.Error("Truncated text should be a prefix of original text")
	}

	// Test case 3: Text exactly at limit should not be truncated
	limitText := strings.Repeat("a", megabyte-1)
	result = truncateStringForTSVectorField(limitText, megabyte)
	if result != limitText {
		t.Error("Text under limit should not be truncated")
	}

	// Test case 4: Mixed Chinese and ASCII text
	mixedText := strings.Repeat("测试Test汉字", megabyte/20) // Create large mixed text
	result = truncateStringForTSVectorField(mixedText, megabyte)

	if len(result) >= megabyte {
		t.Errorf("Mixed text should be truncated under 1MB, got %d bytes", len(result))
	}

	// Verify no broken UTF-8 sequences
	if !strings.HasPrefix(mixedText, result) {
		t.Error("Truncated mixed text should be a valid prefix")
	}

	// Test case 5: Large text ending with ASCII characters
	asciiSuffix := strings.Repeat("a", megabyte-100) + strings.Repeat("测试", 50) + "abcdef"
	result = truncateStringForTSVectorField(asciiSuffix, megabyte)

	if len(result) >= megabyte {
		t.Errorf("ASCII suffix text should be truncated under 1MB, got %d bytes", len(result))
	}

	// Should end with ASCII character
	if !strings.HasPrefix(asciiSuffix, result) {
		t.Error("Truncated ASCII suffix text should be a valid prefix")
	}

	// Test case 6: Large ASCII text to cover ASCII branch in UTF-8 detection
	largeAscii := strings.Repeat("abcdefghijklmnopqrstuvwxyz", megabyte/26+1000)
	result = truncateStringForTSVectorField(largeAscii, megabyte)

	if len(result) >= megabyte {
		t.Errorf("Large ASCII text should be truncated under 1MB, got %d bytes", len(result))
	}

	// Should be a prefix
	if !strings.HasPrefix(largeAscii, result) {
		t.Error("Truncated ASCII text should be a valid prefix")
	}

	// Test case 7: Edge case - string that would trigger the fallback
	// Create a pathological case: all continuation bytes without start bytes
	// This should trigger the fallback because there are no valid UTF-8 boundaries
	invalidBytes := make([]byte, megabyte)
	for i := range invalidBytes {
		invalidBytes[i] = 0x80 // Continuation byte without start byte
	}
	result = truncateStringForTSVectorField(string(invalidBytes), megabyte)

	// Should return empty string as fallback
	if result != "" {
		t.Errorf("Invalid UTF-8 continuation bytes should return empty string, got %d bytes", len(result))
	}
}

// The mark-as-read tests below need a real PostgreSQL database. They are skipped unless
// TEST_MINIFLUX_DATABASE_URL points at a database the tests may migrate and write to.
const skipDatabaseTestsMessage = `Set TEST_MINIFLUX_DATABASE_URL to run the storage database tests`

type markAsReadFixture struct {
	store    *Storage
	user     *model.User
	other    *model.User
	category *model.Category
	feed     *model.Feed
	feed2    *model.Feed
	now      time.Time
}

func newMarkAsReadFixture(t *testing.T) *markAsReadFixture {
	t.Helper()

	databaseURL := os.Getenv("TEST_MINIFLUX_DATABASE_URL")
	if databaseURL == "" {
		t.Skip(skipDatabaseTestsMessage)
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	store := NewStorage(db)
	fixture := &markAsReadFixture{store: store, now: time.Now()}

	fixture.user = createTestUser(t, store)
	fixture.other = createTestUser(t, store)

	fixture.category, err = store.CreateCategory(fixture.user.ID, &model.CategoryCreationRequest{Title: "Mark as read"})
	if err != nil {
		t.Fatal(err)
	}
	otherCategory, err := store.CreateCategory(fixture.user.ID, &model.CategoryCreationRequest{Title: "Other category"})
	if err != nil {
		t.Fatal(err)
	}

	day := 24 * time.Hour
	fixture.feed = createTestFeed(t, store, fixture.user.ID, fixture.category.ID, fixture.now, []time.Duration{40 * day, 31 * day, 2 * day})
	fixture.feed2 = createTestFeed(t, store, fixture.user.ID, otherCategory.ID, fixture.now, []time.Duration{45 * day, 1 * day})

	otherUserCategory, err := store.CreateCategory(fixture.other.ID, &model.CategoryCreationRequest{Title: "Other user"})
	if err != nil {
		t.Fatal(err)
	}
	createTestFeed(t, store, fixture.other.ID, otherUserCategory.ID, fixture.now, []time.Duration{50 * day})

	return fixture
}

func createTestUser(t *testing.T, store *Storage) *model.User {
	t.Helper()
	user, err := store.CreateUser(&model.UserCreationRequest{
		Username: fmt.Sprintf("mark_as_read_%d", rand.Int64()),
		Password: "test-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.RemoveUser(user.ID) })
	return user
}

func createTestFeed(t *testing.T, store *Storage, userID, categoryID int64, now time.Time, ages []time.Duration) *model.Feed {
	t.Helper()
	feedURL := fmt.Sprintf("https://example.org/feed-%d.xml", rand.Int64())
	feed := &model.Feed{
		UserID:   userID,
		Category: &model.Category{ID: categoryID},
		FeedURL:  feedURL,
		SiteURL:  "https://example.org/",
		Title:    feedURL,
	}
	for i, age := range ages {
		feed.Entries = append(feed.Entries, &model.Entry{
			Title:   fmt.Sprintf("Entry %d", i),
			Hash:    fmt.Sprintf("%s-%d", feedURL, i),
			URL:     fmt.Sprintf("%s/entry-%d", feedURL, i),
			Date:    now.Add(-age),
			Content: "content",
			Status:  model.EntryStatusUnread,
		})
	}
	if err := store.CreateFeed(feed); err != nil {
		t.Fatal(err)
	}
	return feed
}

func countUnread(t *testing.T, store *Storage, userID int64, feedID int64) int {
	t.Helper()
	builder := store.NewEntryQueryBuilder(userID).WithStatuses(model.EntryStatusUnread)
	if feedID > 0 {
		builder = builder.WithFeedID(feedID)
	}
	count, err := builder.CountEntries()
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMarkAllAsReadBeforeDateReturnsAffectedCount(t *testing.T) {
	f := newMarkAsReadFixture(t)
	cutoff := f.now.Add(-30 * 24 * time.Hour)

	// Star an old entry: its starred state must survive the bulk update.
	oldEntryID := f.feed.Entries[0].ID
	if err := f.store.SetEntriesStarredState(f.user.ID, []int64{oldEntryID}, true); err != nil {
		t.Fatal(err)
	}

	count, err := f.store.MarkAllAsReadBeforeDate(f.user.ID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 entries marked as read, got %d", count)
	}

	if unread := countUnread(t, f.store, f.user.ID, 0); unread != 2 {
		t.Errorf("expected the 2 recent entries to stay unread, got %d unread", unread)
	}
	if unread := countUnread(t, f.store, f.other.ID, 0); unread != 1 {
		t.Errorf("another user's entries must not change, got %d unread", unread)
	}

	entry, err := f.store.NewEntryQueryBuilder(f.user.ID).WithEntryID(oldEntryID).GetEntry()
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Starred {
		t.Error("starred state must be untouched")
	}

	// A second run with the same cutoff matches nothing.
	count, err = f.store.MarkAllAsReadBeforeDate(f.user.ID, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected an idempotent second run to affect 0 entries, got %d", count)
	}
}

func TestMarkAllAsReadBeforeDateBoundaryIsExclusive(t *testing.T) {
	f := newMarkAsReadFixture(t)
	exact := f.feed.Entries[0].Date

	count, err := f.store.MarkAllAsReadBeforeDate(f.user.ID, exact)
	if err != nil {
		t.Fatal(err)
	}
	// Only the 45-day-old entry is strictly older than the 40-day-old one.
	if count != 1 {
		t.Errorf("expected 1 entry strictly before the cutoff, got %d", count)
	}
}

func TestMarkAllAsReadBeforeDateWithNoMatchIsNoOp(t *testing.T) {
	f := newMarkAsReadFixture(t)

	count, err := f.store.MarkAllAsReadBeforeDate(f.user.ID, f.now.Add(-365*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 entries, got %d", count)
	}
	if unread := countUnread(t, f.store, f.user.ID, 0); unread != 5 {
		t.Errorf("expected all 5 entries to stay unread, got %d", unread)
	}
}

func TestMarkFeedAsReadReturnsAffectedCount(t *testing.T) {
	f := newMarkAsReadFixture(t)

	count, err := f.store.MarkFeedAsRead(f.user.ID, f.feed.ID, f.now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 entries marked as read, got %d", count)
	}
	if unread := countUnread(t, f.store, f.user.ID, f.feed2.ID); unread != 2 {
		t.Errorf("another feed's entries must not change, got %d unread", unread)
	}

	// The feed belongs to another user: nothing changes.
	count, err = f.store.MarkFeedAsRead(f.other.ID, f.feed.ID, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 entries for another user, got %d", count)
	}
}

func TestMarkCategoryAsReadReturnsAffectedCount(t *testing.T) {
	f := newMarkAsReadFixture(t)

	count, err := f.store.MarkCategoryAsRead(f.user.ID, f.category.ID, f.now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 entries marked as read, got %d", count)
	}
	if unread := countUnread(t, f.store, f.user.ID, f.feed2.ID); unread != 2 {
		t.Errorf("another category's entries must not change, got %d unread", unread)
	}

	count, err = f.store.MarkCategoryAsRead(f.user.ID, f.category.ID, f.now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected an idempotent second run to affect 0 entries, got %d", count)
	}
}
