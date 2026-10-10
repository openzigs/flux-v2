// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"miniflux.app/v2/internal/database"
	"miniflux.app/v2/internal/model"
)

// newTestStorage returns a Storage backed by the PostgreSQL database named in
// TEST_MINIFLUX_DATABASE_URL, migrated to the latest schema. The test is
// skipped when the variable is unset, so `go test ./...` needs no database.
func newTestStorage(t testing.TB) *Storage {
	t.Helper()
	dsn := os.Getenv("TEST_MINIFLUX_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_MINIFLUX_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("unable to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(db); err != nil {
		t.Fatalf("unable to migrate database: %v", err)
	}
	return NewStorage(db)
}

func randomSuffix(t testing.TB) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// testFixture is one user with one feed in its first category.
type testFixture struct {
	userID int64
	feedID int64
}

func newTestFixture(t testing.TB, s *Storage) testFixture {
	t.Helper()
	suffix := randomSuffix(t)
	user, err := s.CreateUser(&model.UserCreationRequest{Username: "test_" + suffix, Password: "test123456"})
	if err != nil {
		t.Fatalf("unable to create user: %v", err)
	}
	t.Cleanup(func() { s.RemoveUser(user.ID) })

	category, err := s.FirstCategory(user.ID)
	if err != nil {
		t.Fatalf("unable to get category: %v", err)
	}

	feed := &model.Feed{
		UserID:   user.ID,
		Category: category,
		FeedURL:  "https://example.org/feed-" + suffix + ".xml",
		SiteURL:  "https://example.org/",
		Title:    "Feed " + suffix,
	}
	if err := s.CreateFeed(feed); err != nil {
		t.Fatalf("unable to create feed: %v", err)
	}
	return testFixture{userID: user.ID, feedID: feed.ID}
}

// insertEntry inserts one entry published at the given time and returns its ID.
func insertEntry(t testing.TB, s *Storage, f testFixture, publishedAt time.Time, status string) int64 {
	t.Helper()
	var id int64
	err := s.db.QueryRow(`
		INSERT INTO entries (user_id, feed_id, hash, title, url, comments_url, author, content, published_at, status, changed_at)
		VALUES ($1, $2, $3, 'title', 'https://example.org/entry', '', '', 'content', $4, $5, now() - interval '1 day')
		RETURNING id`,
		f.userID, f.feedID, randomSuffix(t), publishedAt, status,
	).Scan(&id)
	if err != nil {
		t.Fatalf("unable to insert entry: %v", err)
	}
	return id
}

func entryStatusAndChangedAt(t testing.TB, s *Storage, entryID int64) (string, time.Time) {
	t.Helper()
	var status string
	var changedAt time.Time
	if err := s.db.QueryRow(`SELECT status, changed_at FROM entries WHERE id=$1`, entryID).Scan(&status, &changedAt); err != nil {
		t.Fatalf("unable to read entry %d: %v", entryID, err)
	}
	return status, changedAt
}

func countUnread(t testing.TB, s *Storage, userID int64) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM entries WHERE user_id=$1 AND status=$2`, userID, model.EntryStatusUnread).Scan(&n); err != nil {
		t.Fatalf("unable to count unread entries: %v", err)
	}
	return n
}
