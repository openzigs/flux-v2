// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"miniflux.app/v2/internal/database"
	"miniflux.app/v2/internal/model"
)

// These tests need a disposable PostgreSQL database. They skip unless
// TEST_MINIFLUX_DATABASE_URL is set, e.g.
// postgres://user:pass@localhost:5432/miniflux_test?sslmode=disable
const skipStorageTestsMessage = `Set TEST_MINIFLUX_DATABASE_URL to run the storage integration tests`

var migrateOnce sync.Once

type markAsReadFixture struct {
	t     *testing.T
	store *Storage
	db    *sql.DB
	seq   int
}

func newMarkAsReadFixture(t *testing.T) *markAsReadFixture {
	t.Helper()
	dsn := os.Getenv("TEST_MINIFLUX_DATABASE_URL")
	if dsn == "" {
		t.Skip(skipStorageTestsMessage)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	var migrateErr error
	migrateOnce.Do(func() { migrateErr = database.Migrate(db) })
	if migrateErr != nil {
		t.Fatal(migrateErr)
	}

	return &markAsReadFixture{t: t, store: NewStorage(db), db: db}
}

func (f *markAsReadFixture) user() int64 {
	f.t.Helper()
	f.seq++
	u, err := f.store.CreateUser(&model.UserCreationRequest{
		Username: fmt.Sprintf("mark_read_%d_%d", time.Now().UnixNano(), f.seq),
		Password: "test-password-123",
	})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { f.store.RemoveUser(u.ID) })
	return u.ID
}

func (f *markAsReadFixture) category(userID int64, hidden bool) int64 {
	f.t.Helper()
	f.seq++
	c, err := f.store.CreateCategory(userID, &model.CategoryCreationRequest{
		Title:        fmt.Sprintf("category %d", f.seq),
		HideGlobally: hidden,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return c.ID
}

func (f *markAsReadFixture) feed(userID, categoryID int64, hidden bool) int64 {
	f.t.Helper()
	f.seq++
	var id int64
	err := f.db.QueryRow(
		`INSERT INTO feeds (user_id, category_id, title, feed_url, site_url, hide_globally)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, categoryID, fmt.Sprintf("feed %d", f.seq),
		fmt.Sprintf("https://example.org/feed/%d.xml", f.seq), "https://example.org/", hidden,
	).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

// entry inserts an entry published at the given instant with the given status and starred flag.
func (f *markAsReadFixture) entry(userID, feedID int64, publishedAt time.Time, status string, starred bool) int64 {
	f.t.Helper()
	f.seq++
	var id int64
	err := f.db.QueryRow(
		`INSERT INTO entries (user_id, feed_id, hash, published_at, title, url, status, starred, changed_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now()) RETURNING id`,
		userID, feedID, fmt.Sprintf("hash-%d-%d", time.Now().UnixNano(), f.seq), publishedAt,
		"title", "https://example.org/entry", status, starred,
	).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *markAsReadFixture) entries(n int, userID, feedID int64, publishedAt time.Time) []int64 {
	ids := make([]int64, 0, n)
	for range n {
		ids = append(ids, f.entry(userID, feedID, publishedAt, model.EntryStatusUnread, false))
	}
	return ids
}

func (f *markAsReadFixture) status(entryID int64) (status string, starred bool) {
	f.t.Helper()
	if err := f.db.QueryRow(`SELECT status, starred FROM entries WHERE id=$1`, entryID).Scan(&status, &starred); err != nil {
		f.t.Fatal(err)
	}
	return status, starred
}

func (f *markAsReadFixture) unreadCount(userID int64) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM entries WHERE user_id=$1 AND status='unread'`, userID).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}
