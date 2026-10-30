// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"miniflux.app/v2/internal/database"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

// These handler tests need a disposable PostgreSQL database and skip unless
// TEST_MINIFLUX_DATABASE_URL is set.
var uiMigrateOnce sync.Once

type uiFixture struct {
	t   *testing.T
	db  *sql.DB
	h   *handler
	seq int
}

func newUIFixture(t *testing.T) *uiFixture {
	t.Helper()
	dsn := os.Getenv("TEST_MINIFLUX_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set TEST_MINIFLUX_DATABASE_URL to run the UI mark-as-read handler tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var migrateErr error
	uiMigrateOnce.Do(func() { migrateErr = database.Migrate(db) })
	if migrateErr != nil {
		t.Fatal(migrateErr)
	}
	return &uiFixture{t: t, db: db, h: &handler{store: storage.NewStorage(db)}}
}

func (f *uiFixture) user() int64 {
	f.t.Helper()
	f.seq++
	u, err := f.h.store.CreateUser(&model.UserCreationRequest{
		Username: fmt.Sprintf("ui_mark_read_%d_%d", time.Now().UnixNano(), f.seq),
		Password: "test-password-123",
	})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { f.h.store.RemoveUser(u.ID) })
	return u.ID
}

func (f *uiFixture) category(userID int64, hidden bool) int64 {
	f.t.Helper()
	f.seq++
	c, err := f.h.store.CreateCategory(userID, &model.CategoryCreationRequest{Title: fmt.Sprintf("c%d", f.seq), HideGlobally: hidden})
	if err != nil {
		f.t.Fatal(err)
	}
	return c.ID
}

func (f *uiFixture) feed(userID, categoryID int64, hidden bool) int64 {
	f.t.Helper()
	f.seq++
	var id int64
	if err := f.db.QueryRow(
		`INSERT INTO feeds (user_id, category_id, title, feed_url, site_url, hide_globally, checked_at)
		 VALUES ($1, $2, 'f', $3, 'https://example.org/', $4, now()) RETURNING id`,
		userID, categoryID, fmt.Sprintf("https://example.org/ui/%d.xml", f.seq), hidden,
	).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *uiFixture) entries(n int, userID, feedID int64, publishedAt time.Time) []int64 {
	f.t.Helper()
	ids := make([]int64, 0, n)
	for range n {
		f.seq++
		var id int64
		if err := f.db.QueryRow(
			`INSERT INTO entries (user_id, feed_id, hash, published_at, title, url, status, changed_at, created_at)
			 VALUES ($1, $2, $3, $4, 't', 'https://example.org/e', 'unread', now(), now()) RETURNING id`,
			userID, feedID, fmt.Sprintf("ui-%d-%d", time.Now().UnixNano(), f.seq), publishedAt,
		).Scan(&id); err != nil {
			f.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func (f *uiFixture) unread(userID int64) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM entries WHERE user_id=$1 AND status='unread'`, userID).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// post calls a UI handler as the given user, with routeParams set as path values.
func (f *uiFixture) post(fn http.HandlerFunc, userID int64, target string, routeParams map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, target, nil)
	for k, v := range routeParams {
		r.SetPathValue(k, v)
	}
	r = r.WithContext(context.WithValue(r.Context(), request.UserIDContextKey, userID))
	w := httptest.NewRecorder()
	fn(w, r)
	return w
}
