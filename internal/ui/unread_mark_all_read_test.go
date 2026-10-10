// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/model"
)

func newMarkOlderThanRequest(t *testing.T, form url.Values) (*http.Request, *model.WebSession) {
	t.Helper()
	sess, _ := model.NewWebSession("test-agent", "127.0.0.1")
	r := httptest.NewRequest(http.MethodPost, "/mark-older-than-as-read", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(r.Context(), request.WebSessionContextKey, sess)
	ctx = context.WithValue(ctx, request.UserIDContextKey, int64(1))
	return r.WithContext(ctx), sess
}

func TestMarkOlderThanAsReadRejectsInvalidAge(t *testing.T) {
	for _, value := range []string{"0", "-1", "abc", "", " ", "1.5", "36501"} {
		t.Run(value, func(t *testing.T) {
			// A nil store makes any storage call panic: rejection must happen first.
			h := &handler{basePath: "", store: nil}
			r, sess := newMarkOlderThanRequest(t, url.Values{"days": {value}})
			w := httptest.NewRecorder()

			h.markOlderThanAsRead(w, r)

			if w.Code != http.StatusFound {
				t.Fatalf("expected a redirect, got status %d", w.Code)
			}
			if location := w.Header().Get("Location"); location != "/unread" {
				t.Fatalf("expected a redirect to /unread, got %q", location)
			}
			success, failure := sess.ConsumeMessages()
			if failure == "" {
				t.Fatal("expected an explicit error message")
			}
			if success != "" {
				t.Fatalf("expected no success message, got %q", success)
			}
		})
	}
}

func TestMarkOlderThanAsReadRejectsMissingField(t *testing.T) {
	h := &handler{basePath: "/base", store: nil}
	r, sess := newMarkOlderThanRequest(t, url.Values{})
	w := httptest.NewRecorder()

	h.markOlderThanAsRead(w, r)

	if location := w.Header().Get("Location"); location != "/base/unread" {
		t.Fatalf("expected a redirect to /base/unread, got %q", location)
	}
	if _, failure := sess.ConsumeMessages(); failure == "" {
		t.Fatal("expected an explicit error message")
	}
}

func TestParseAgeInDays(t *testing.T) {
	for value, want := range map[string]int{"1": 1, "30": 30, " 7 ": 7, "36500": 36500} {
		got, err := parseAgeInDays(value)
		if err != nil || got != want {
			t.Errorf("parseAgeInDays(%q) = %d, %v; want %d", value, got, err, want)
		}
	}
}

func TestAgeCutoffUsesFixedDays(t *testing.T) {
	// 2026-03-08 is a DST change in America/New_York: a fixed-duration cutoff
	// is exactly n*24h earlier regardless.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("time zone database unavailable")
	}
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, loc)
	if got := ageCutoff(now, 3); now.Sub(got) != 72*time.Hour {
		t.Fatalf("expected a 72h cutoff, got %v", now.Sub(got))
	}
}

func contextWithUser(r *http.Request, sess *model.WebSession, userID int64) context.Context {
	ctx := context.WithValue(r.Context(), request.WebSessionContextKey, sess)
	return context.WithValue(ctx, request.UserIDContextKey, userID)
}
