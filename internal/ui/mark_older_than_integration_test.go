// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
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
		INSERT INTO entries (user_id, feed_id, hash, title, url, comments_url, author, content, published_at, status, changed_at)
		VALUES ($1, $2, $3, 'title', 'https://example.org/entry', '', '', 'content', now() - $4::interval, $5, now() - interval '1 day')
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

func TestMarkOlderThanOnlyTouchesTheCurrentUser(t *testing.T) {
	env := newMarkReadEnv(t)
	a := env.newUser(t)
	b := env.newUser(t)
	day := 24 * time.Hour

	aStale := env.insertEntry(t, a, 60*day, model.EntryStatusUnread)
	bStale := env.insertEntry(t, b, 60*day, model.EntryStatusUnread)
	env.insertEntry(t, b, 90*day, model.EntryStatusUnread)
	bUnreadBefore := env.countUnread(t, b.id)

	if _, failure := env.markOlderThan(t, a.id, "30"); failure != "" {
		t.Fatalf("unexpected error message %q", failure)
	}
	if got := env.status(t, aStale); got != model.EntryStatusRead {
		t.Errorf("user A's stale entry: expected read, got %q", got)
	}
	if got := env.status(t, bStale); got != model.EntryStatusUnread {
		t.Errorf("user B's stale entry: expected unread, got %q", got)
	}
	if got := env.countUnread(t, b.id); got != bUnreadBefore {
		t.Errorf("user B's unread count changed from %d to %d", bUnreadBefore, got)
	}
}

func TestMarkOlderThanKeepsStarredAndSharedEntriesIntact(t *testing.T) {
	env := newMarkReadEnv(t)
	u := env.newUser(t)

	entryID := env.insertEntry(t, u, 60*24*time.Hour, model.EntryStatusUnread)
	if err := env.store.SetEntriesStarredState(u.id, []int64{entryID}, true); err != nil {
		t.Fatal(err)
	}
	shareCode, err := env.store.EntryShareCode(u.id, entryID)
	if err != nil || shareCode == "" {
		t.Fatalf("unable to share entry: %q, %v", shareCode, err)
	}
	before, err := env.store.NewEntryQueryBuilder(u.id).WithEntryIDs(entryID).GetEntry()
	if err != nil || before == nil {
		t.Fatalf("unable to load entry: %v", err)
	}

	if _, failure := env.markOlderThan(t, u.id, "30"); failure != "" {
		t.Fatalf("unexpected error message %q", failure)
	}

	after, err := env.store.NewEntryQueryBuilder(u.id).WithEntryIDs(entryID).GetEntry()
	if err != nil || after == nil {
		t.Fatalf("unable to reload entry: %v", err)
	}
	if after.Status != model.EntryStatusRead {
		t.Errorf("expected read, got %q", after.Status)
	}
	if !after.Starred {
		t.Error("expected the entry to stay starred")
	}
	if after.ShareCode != shareCode || after.Title != before.Title || after.Content != before.Content {
		t.Errorf("share code, title or content changed: %+v", after)
	}
	shared, err := env.store.NewAnonymousQueryBuilder().WithShareCode(shareCode).GetEntry()
	if err != nil || shared == nil || shared.ID != entryID {
		t.Fatalf("share code no longer resolves to the entry: %v, %v", shared, err)
	}
}

func (env *markReadEnv) changedAt(t testing.TB, entryID int64) time.Time {
	t.Helper()
	var changedAt time.Time
	if err := env.db.QueryRow(`SELECT changed_at FROM entries WHERE id=$1`, entryID).Scan(&changedAt); err != nil {
		t.Fatalf("unable to read entry %d: %v", entryID, err)
	}
	return changedAt
}

func TestMarkOlderThanRepeatReportsZero(t *testing.T) {
	env := newMarkReadEnv(t)
	u := env.newUser(t)
	day := 24 * time.Hour

	alreadyRead := env.insertEntry(t, u, 90*day, model.EntryStatusRead)
	env.insertEntry(t, u, 60*day, model.EntryStatusUnread)
	readChangedAt := env.changedAt(t, alreadyRead)

	if success, _ := env.markOlderThan(t, u.id, "30"); success != "1 entries marked as read." {
		t.Fatalf("first run: unexpected success message %q", success)
	}
	firstRunChangedAt := env.changedAt(t, alreadyRead)

	success, failure := env.markOlderThan(t, u.id, "30")
	if failure != "" {
		t.Fatalf("second run: unexpected error message %q", failure)
	}
	if success != "0 entries marked as read." {
		t.Fatalf("second run: unexpected success message %q", success)
	}
	if got := env.changedAt(t, alreadyRead); !got.Equal(readChangedAt) || !got.Equal(firstRunChangedAt) {
		t.Fatalf("already-read entry changed_at moved from %v to %v", readChangedAt, got)
	}
}

func TestMarkOlderThanReportedCountMatchesUnreadDelta(t *testing.T) {
	env := newMarkReadEnv(t)
	u := env.newUser(t)
	day := 24 * time.Hour

	for _, age := range []time.Duration{8 * day, 10 * day, 20 * day, 200 * day} {
		env.insertEntry(t, u, age, model.EntryStatusUnread)
	}
	env.insertEntry(t, u, 2*day, model.EntryStatusUnread)
	env.insertEntry(t, u, 30*day, model.EntryStatusRead)
	unreadBefore := env.countUnread(t, u.id)

	sess, _ := model.NewWebSession("test-agent", "127.0.0.1")
	r := httptest.NewRequest(http.MethodPost, "/mark-older-than-as-read", nil)
	r.PostForm = url.Values{"days": {"7"}}
	r = r.WithContext(contextWithUser(r, sess, u.id))
	w := httptest.NewRecorder()
	env.h.markOlderThanAsRead(w, r)

	// The handler redirects to the unread page, which re-renders its counter
	// from the database: no manual refresh is needed.
	if location := w.Header().Get("Location"); location != "/unread" {
		t.Fatalf("expected a redirect to /unread, got %q", location)
	}
	success, _ := sess.ConsumeMessages()
	var reported int
	if _, err := fmt.Sscanf(success, "%d entries marked as read.", &reported); err != nil {
		t.Fatalf("unable to read the count from %q: %v", success, err)
	}
	if reported != 4 {
		t.Fatalf("expected 4 entries reported, got %d", reported)
	}
	if delta := unreadBefore - env.countUnread(t, u.id); delta != reported {
		t.Fatalf("unread count dropped by %d, but %d were reported", delta, reported)
	}
}

func TestMarkOlderThanLargeVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("large-volume test skipped in -short mode")
	}
	env := newMarkReadEnv(t)
	u := env.newUser(t)
	const stale = 100_000

	_, err := env.db.Exec(`
		INSERT INTO entries (user_id, feed_id, hash, title, url, comments_url, author, content, published_at, status, changed_at)
		SELECT $1, $2, 'bulk-' || g, 'title', 'https://example.org/entry', '', '', 'content',
			now() - interval '60 days' - (g || ' seconds')::interval, 'unread', now()
		FROM generate_series(1, $3) AS g`,
		u.id, u.feedID, stale,
	)
	if err != nil {
		t.Fatalf("unable to bulk insert entries: %v", err)
	}
	fresh := env.insertEntry(t, u, time.Hour, model.EntryStatusUnread)
	unreadBefore := env.countUnread(t, u.id)

	start := time.Now()
	success, failure := env.markOlderThan(t, u.id, "30")
	elapsed := time.Since(start)
	t.Logf("marked %d entries in %v", stale, elapsed)

	if failure != "" {
		t.Fatalf("unexpected error message %q", failure)
	}
	if want := fmt.Sprintf("%d entries marked as read.", stale); success != want {
		t.Fatalf("expected %q, got %q", want, success)
	}
	if delta := unreadBefore - env.countUnread(t, u.id); delta != stale {
		t.Fatalf("expected the unread count to drop by %d, got %d", stale, delta)
	}
	if got := env.status(t, fresh); got != model.EntryStatusUnread {
		t.Fatalf("fresh entry: expected unread, got %q", got)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("marking %d entries took %v, over the 5s budget", stale, elapsed)
	}
}
