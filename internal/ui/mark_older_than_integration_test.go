// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"miniflux.app/v2/internal/database"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

// These tests drive markOlderThanAsRead against a real PostgreSQL database.
// They skip unless TEST_MINIFLUX_DATABASE_URL is set, so `go test ./...`
// needs no database.

type markReadEnv struct {
	db    *sql.DB
	store *storage.Storage
	h     *handler
}

type markReadUser struct {
	id     int64
	feedID int64
}

func newMarkReadEnv(t testing.TB) *markReadEnv {
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
	store := storage.NewStorage(db)
	return &markReadEnv{db: db, store: store, h: &handler{basePath: "", store: store}}
}

func randomTestSuffix(t testing.TB) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func (env *markReadEnv) newUser(t testing.TB) markReadUser {
	t.Helper()
	suffix := randomTestSuffix(t)
	user, err := env.store.CreateUser(&model.UserCreationRequest{Username: "ui_test_" + suffix, Password: "test123456"})
	if err != nil {
		t.Fatalf("unable to create user: %v", err)
	}
	t.Cleanup(func() { env.store.RemoveUser(user.ID) })
	category, err := env.store.FirstCategory(user.ID)
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
	if err := env.store.CreateFeed(feed); err != nil {
		t.Fatalf("unable to create feed: %v", err)
	}
	return markReadUser{id: user.ID, feedID: feed.ID}
}

func (env *markReadEnv) insertEntry(t testing.TB, u markReadUser, age time.Duration, status string) int64 {
	t.Helper()
	var id int64
	err := env.db.QueryRow(`
		INSERT INTO entries (user_id, feed_id, hash, title, url, content, published_at, status, changed_at)
		VALUES ($1, $2, $3, 'title', 'https://example.org/entry', 'content', now() - $4::interval, $5, now() - interval '1 day')
		RETURNING id`,
		u.id, u.feedID, randomTestSuffix(t), age.String(), status,
	).Scan(&id)
	if err != nil {
		t.Fatalf("unable to insert entry: %v", err)
	}
	return id
}

func (env *markReadEnv) status(t testing.TB, entryID int64) string {
	t.Helper()
	var status string
	if err := env.db.QueryRow(`SELECT status FROM entries WHERE id=$1`, entryID).Scan(&status); err != nil {
		t.Fatalf("unable to read entry %d: %v", entryID, err)
	}
	return status
}

func (env *markReadEnv) countUnread(t testing.TB, userID int64) int {
	t.Helper()
	n, err := env.store.NewEntryQueryBuilder(userID).WithStatuses(model.EntryStatusUnread).CountEntries()
	if err != nil {
		t.Fatalf("unable to count unread entries: %v", err)
	}
	return n
}

// markOlderThan posts the form as the given user and returns the success and error flash messages.
func (env *markReadEnv) markOlderThan(t testing.TB, userID int64, days string) (string, string) {
	t.Helper()
	sess, _ := model.NewWebSession("test-agent", "127.0.0.1")
	r := httptest.NewRequest(http.MethodPost, "/mark-older-than-as-read", nil)
	r.PostForm = url.Values{"days": {days}}
	r = r.WithContext(contextWithUser(r, sess, userID))
	w := httptest.NewRecorder()
	env.h.markOlderThanAsRead(w, r)
	if w.Code != http.StatusFound {
		t.Fatalf("expected a redirect, got %d: %s", w.Code, w.Body.String())
	}
	return sess.ConsumeMessages()
}

func TestMarkOlderThanSelectsOnlyStaleEntries(t *testing.T) {
	env := newMarkReadEnv(t)
	u := env.newUser(t)
	day := 24 * time.Hour

	stale := []int64{
		env.insertEntry(t, u, 30*day+time.Minute, model.EntryStatusUnread),
		env.insertEntry(t, u, 45*day, model.EntryStatusUnread),
		env.insertEntry(t, u, 400*day, model.EntryStatusUnread),
	}
	fresh := []int64{
		env.insertEntry(t, u, 30*day-time.Minute, model.EntryStatusUnread),
		env.insertEntry(t, u, day, model.EntryStatusUnread),
		env.insertEntry(t, u, time.Minute, model.EntryStatusUnread),
	}

	success, failure := env.markOlderThan(t, u.id, "30")
	if failure != "" {
		t.Fatalf("unexpected error message %q", failure)
	}
	if success == "" {
		t.Fatal("expected a success message")
	}
	for _, id := range stale {
		if got := env.status(t, id); got != model.EntryStatusRead {
			t.Errorf("stale entry %d: expected read, got %q", id, got)
		}
	}
	for _, id := range fresh {
		if got := env.status(t, id); got != model.EntryStatusUnread {
			t.Errorf("fresh entry %d: expected unread, got %q", id, got)
		}
	}
}
