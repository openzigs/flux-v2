// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api // import "miniflux.app/v2/internal/api"

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	miniflux "miniflux.app/v2/client"
)

type markAsReadDaysResponse struct {
	Count int64 `json:"count"`
}

// putMarkAsRead sends a raw PUT request, because the Go client has no "days" variant.
func putMarkAsRead(t *testing.T, baseURL, username, password, path string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPut, strings.TrimSuffix(baseURL, "/")+"/v1"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.SetBasicAuth(username, password)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body
}

func decodeMarkAsReadCount(t *testing.T, status int, body []byte) int64 {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", status, body)
	}
	var result markAsReadDaysResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON body %q: %v", body, err)
	}
	return result.Count
}

type markAsReadDaysFixture struct {
	config   *integrationTestConfig
	admin    *miniflux.Client
	user     *miniflux.User
	client   *miniflux.Client
	feedID   int64
	category *miniflux.Category
	entries  miniflux.Entries
}

func newMarkAsReadDaysFixture(t *testing.T) *markAsReadDaysFixture {
	t.Helper()
	testConfig := newIntegrationTestConfig()
	if !testConfig.isConfigured() {
		t.Skip(skipIntegrationTestsMessage)
	}

	adminClient := miniflux.NewClient(testConfig.testBaseURL, testConfig.testAdminUsername, testConfig.testAdminPassword)
	user, err := adminClient.CreateUser(testConfig.genRandomUsername(), testConfig.testRegularPassword, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminClient.DeleteUser(user.ID) })

	client := miniflux.NewClient(testConfig.testBaseURL, user.Username, testConfig.testRegularPassword)
	category, err := client.CreateCategory("Mark as read days")
	if err != nil {
		t.Fatal(err)
	}
	feedID, err := client.CreateFeed(&miniflux.FeedCreationRequest{FeedURL: testConfig.testFeedURL, CategoryID: category.ID})
	if err != nil {
		t.Fatal(err)
	}
	results, err := client.FeedEntries(feedID, &miniflux.Filter{Status: miniflux.EntryStatusUnread})
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Entries) == 0 {
		t.Fatal("the test feed must have entries")
	}

	return &markAsReadDaysFixture{config: testConfig, admin: adminClient, user: user, client: client, feedID: feedID, category: category, entries: results.Entries}
}

func (f *markAsReadDaysFixture) put(t *testing.T, path string) (int, []byte) {
	return putMarkAsRead(t, f.config.testBaseURL, f.user.Username, f.config.testRegularPassword, path)
}

func (f *markAsReadDaysFixture) putCount(t *testing.T, path string) int64 {
	t.Helper()
	status, body := f.put(t, path)
	return decodeMarkAsReadCount(t, status, body)
}

func (f *markAsReadDaysFixture) unreadCount(t *testing.T) int {
	t.Helper()
	results, err := f.client.FeedEntries(f.feedID, &miniflux.Filter{Status: miniflux.EntryStatusUnread})
	if err != nil {
		t.Fatal(err)
	}
	return results.Total
}

// medianDays returns a number of days that splits the feed's entries, and how many entries
// are strictly older than that many days.
func (f *markAsReadDaysFixture) medianDays() (int, int) {
	median := f.entries[len(f.entries)/2]
	days := int(time.Since(median.Date).Hours() / 24)
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	older := 0
	for _, entry := range f.entries {
		if entry.Date.Before(cutoff) {
			older++
		}
	}
	return days, older
}

func TestMarkUserAsReadWithDays(t *testing.T) {
	t.Parallel()
	f := newMarkAsReadDaysFixture(t)
	path := fmt.Sprintf("/users/%d/mark-all-as-read", f.user.ID)
	total := f.unreadCount(t)

	// Nothing is older than 100 years: zero-match reports 0.
	if count := f.putCount(t, path+"?days=36500"); count != 0 {
		t.Fatalf("expected 0 entries older than 36500 days, got %d", count)
	}

	days, older := f.medianDays()
	count := f.putCount(t, fmt.Sprintf("%s?days=%d", path, days))
	if count != int64(older) {
		t.Errorf("expected %d entries older than %d days, got %d", older, days, count)
	}
	if unread := f.unreadCount(t); unread != total-int(count) {
		t.Errorf("recent entries must stay unread: expected %d unread, got %d", total-int(count), unread)
	}

	// Re-running with the same N is idempotent.
	if count := f.putCount(t, fmt.Sprintf("%s?days=%d", path, days)); count != 0 {
		t.Errorf("expected a second run to affect 0 entries, got %d", count)
	}
}

func TestMarkUserAsReadWithInvalidDays(t *testing.T) {
	t.Parallel()
	f := newMarkAsReadDaysFixture(t)
	path := fmt.Sprintf("/users/%d/mark-all-as-read", f.user.ID)
	total := f.unreadCount(t)

	for _, query := range []string{"?days=", "?days=-5", "?days=abc", "?days=1.5"} {
		if status, body := f.put(t, path+query); status != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", query, status, body)
		}
	}
	if unread := f.unreadCount(t); unread != total {
		t.Errorf("an invalid request must change nothing: expected %d unread, got %d", total, unread)
	}
}

func TestMarkUserAsReadWithoutDaysIsUnchanged(t *testing.T) {
	t.Parallel()
	f := newMarkAsReadDaysFixture(t)

	status, body := f.put(t, fmt.Sprintf("/users/%d/mark-all-as-read", f.user.ID))
	if status != http.StatusNoContent || len(body) != 0 {
		t.Fatalf("expected 204 with no body, got %d: %q", status, body)
	}
	if unread := f.unreadCount(t); unread != 0 {
		t.Errorf("expected every entry read, got %d unread", unread)
	}
}

func TestMarkUserAsReadWithDaysAsOtherUser(t *testing.T) {
	t.Parallel()
	f := newMarkAsReadDaysFixture(t)
	adminUser, err := f.admin.Me()
	if err != nil {
		t.Fatal(err)
	}

	if status, _ := f.put(t, fmt.Sprintf("/users/%d/mark-all-as-read?days=0", adminUser.ID)); status != http.StatusForbidden {
		t.Errorf("expected 403 for another user, got %d", status)
	}
}

// testMarkScopeAsReadWithDays covers a scoped mark-all-as-read endpoint (feed or category)
// with and without the days parameter.
func testMarkScopeAsReadWithDays(t *testing.T, pathFor func(f *markAsReadDaysFixture) string) {
	t.Run("days marks only older entries", func(t *testing.T) {
		t.Parallel()
		f := newMarkAsReadDaysFixture(t)
		path := pathFor(f)
		total := f.unreadCount(t)

		days, older := f.medianDays()
		count := f.putCount(t, fmt.Sprintf("%s?days=%d", path, days))
		if count != int64(older) {
			t.Errorf("expected %d entries older than %d days, got %d", older, days, count)
		}
		if unread := f.unreadCount(t); unread != total-int(count) {
			t.Errorf("newer entries must stay unread: expected %d unread, got %d", total-int(count), unread)
		}
		if count := f.putCount(t, fmt.Sprintf("%s?days=%d", path, days)); count != 0 {
			t.Errorf("expected a second run to affect 0 entries, got %d", count)
		}
	})

	t.Run("invalid days is rejected", func(t *testing.T) {
		t.Parallel()
		f := newMarkAsReadDaysFixture(t)
		path := pathFor(f)
		total := f.unreadCount(t)

		for _, query := range []string{"?days=", "?days=-1", "?days=abc"} {
			if status, body := f.put(t, path+query); status != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d: %s", query, status, body)
			}
		}
		if unread := f.unreadCount(t); unread != total {
			t.Errorf("an invalid request must change nothing: expected %d unread, got %d", total, unread)
		}
	})

	t.Run("absent days is unchanged", func(t *testing.T) {
		t.Parallel()
		f := newMarkAsReadDaysFixture(t)

		status, body := f.put(t, pathFor(f))
		if status != http.StatusNoContent || len(body) != 0 {
			t.Fatalf("expected 204 with no body, got %d: %q", status, body)
		}
		if unread := f.unreadCount(t); unread != 0 {
			t.Errorf("expected every entry read, got %d unread", unread)
		}
	})
}

func TestMarkFeedAsReadWithDays(t *testing.T) {
	testMarkScopeAsReadWithDays(t, func(f *markAsReadDaysFixture) string {
		return fmt.Sprintf("/feeds/%d/mark-all-as-read", f.feedID)
	})
}
