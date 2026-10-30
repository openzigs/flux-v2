// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api // import "miniflux.app/v2/internal/api"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	miniflux "miniflux.app/v2/client"
)

// AC-10: without older_than_days the REST mark-all action keeps its 204 and marks everything.
func TestMarkUserAsReadWithoutAgeIsUnchanged(t *testing.T) {
	testConfig := newIntegrationTestConfig()
	if !testConfig.isConfigured() {
		t.Skip(skipIntegrationTestsMessage)
	}

	adminClient := miniflux.NewClient(testConfig.testBaseURL, testConfig.testAdminUsername, testConfig.testAdminPassword)
	user, err := adminClient.CreateUser(testConfig.genRandomUsername(), testConfig.testRegularPassword, false)
	if err != nil {
		t.Fatal(err)
	}
	defer adminClient.DeleteUser(user.ID)

	client := miniflux.NewClient(testConfig.testBaseURL, user.Username, testConfig.testRegularPassword)
	feedID, err := client.CreateFeed(&miniflux.FeedCreationRequest{FeedURL: testConfig.testFeedURL})
	if err != nil {
		t.Fatal(err)
	}

	if err := client.MarkAllAsRead(user.ID); err != nil {
		t.Fatal(err)
	}

	results, err := client.FeedEntries(feedID, &miniflux.Filter{Status: miniflux.EntryStatusUnread})
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 0 {
		t.Fatalf("unread after mark-all = %d, want 0", results.Total)
	}
}

// putMarkAllAsRead calls a mark-all-as-read endpoint directly so the raw status and body can be asserted.
func putMarkAllAsRead(t *testing.T, cfg *integrationTestConfig, username, path, olderThanDays string) (int, map[string]int64) {
	t.Helper()
	u := cfg.testBaseURL + path
	if olderThanDays != "" {
		u += "?older_than_days=" + url.QueryEscape(olderThanDays)
	}
	req, err := http.NewRequest(http.MethodPut, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth(username, cfg.testRegularPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var body map[string]int64
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode, body
}

func newMarkAsReadTestUser(t *testing.T, cfg *integrationTestConfig) (*miniflux.User, *miniflux.Client, int64) {
	t.Helper()
	adminClient := miniflux.NewClient(cfg.testBaseURL, cfg.testAdminUsername, cfg.testAdminPassword)
	user, err := adminClient.CreateUser(cfg.genRandomUsername(), cfg.testRegularPassword, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminClient.DeleteUser(user.ID) })

	client := miniflux.NewClient(cfg.testBaseURL, user.Username, cfg.testRegularPassword)
	feedID, err := client.CreateFeed(&miniflux.FeedCreationRequest{FeedURL: cfg.testFeedURL})
	if err != nil {
		t.Fatal(err)
	}
	return user, client, feedID
}

func unreadTotal(t *testing.T, client *miniflux.Client) int {
	t.Helper()
	results, err := client.Entries(&miniflux.Filter{Status: miniflux.EntryStatusUnread})
	if err != nil {
		t.Fatal(err)
	}
	return results.Total
}

func TestMarkUserAsReadOlderThanDays(t *testing.T) {
	cfg := newIntegrationTestConfig()
	if !cfg.isConfigured() {
		t.Skip(skipIntegrationTestsMessage)
	}

	user, client, _ := newMarkAsReadTestUser(t, cfg)
	other, otherClient, _ := newMarkAsReadTestUser(t, cfg)
	path := fmt.Sprintf("/v1/users/%d/mark-all-as-read", user.ID)

	// AC-6: invalid values are rejected and change nothing.
	before := unreadTotal(t, client)
	for _, value := range []string{"0", "-5", "abc"} {
		if status, _ := putMarkAllAsRead(t, cfg, user.Username, path, value); status != http.StatusBadRequest {
			t.Errorf("older_than_days=%s: status %d, want 400", value, status)
		}
	}
	if got := unreadTotal(t, client); got != before {
		t.Fatalf("unread changed after rejected requests: %d -> %d", before, got)
	}

	// AC-1/AC-4: a valid value reports the changed count, and the unread view reflects it (AC-9).
	status, body := putMarkAllAsRead(t, cfg, user.Username, path, "1")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
	changed, ok := body["changed_count"]
	if !ok {
		t.Fatalf("response has no changed_count: %v", body)
	}
	if got := unreadTotal(t, client); int64(before-got) != changed {
		t.Fatalf("unread went %d -> %d but changed_count = %d", before, got, changed)
	}

	// AC-5: idempotent.
	if _, body := putMarkAllAsRead(t, cfg, user.Username, path, "1"); body["changed_count"] != 0 {
		t.Fatalf("second run changed %d, want 0", body["changed_count"])
	}

	// AC-11: the other user's entries are untouched, and a user cannot target another user.
	if unreadTotal(t, otherClient) == 0 {
		t.Fatal("other user's entries were marked as read")
	}
	if status, _ := putMarkAllAsRead(t, cfg, user.Username, fmt.Sprintf("/v1/users/%d/mark-all-as-read", other.ID), "1"); status != http.StatusForbidden {
		t.Fatalf("cross-user status %d, want 403", status)
	}
}

func TestMarkFeedAsReadOlderThanDays(t *testing.T) {
	cfg := newIntegrationTestConfig()
	if !cfg.isConfigured() {
		t.Skip(skipIntegrationTestsMessage)
	}

	user, client, feedID := newMarkAsReadTestUser(t, cfg)
	path := fmt.Sprintf("/v1/feeds/%d/mark-all-as-read", feedID)

	if status, _ := putMarkAllAsRead(t, cfg, user.Username, path, "abc"); status != http.StatusBadRequest {
		t.Fatalf("invalid value: status %d, want 400", status)
	}

	before := unreadTotal(t, client)
	status, body := putMarkAllAsRead(t, cfg, user.Username, path, "1")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
	if got := unreadTotal(t, client); int64(before-got) != body["changed_count"] {
		t.Fatalf("unread went %d -> %d but changed_count = %d", before, got, body["changed_count"])
	}

	// AC-10: without the parameter the endpoint keeps its 204 and marks the whole feed.
	if status, _ := putMarkAllAsRead(t, cfg, user.Username, path, ""); status != http.StatusNoContent {
		t.Fatalf("status %d, want 204", status)
	}
	if got := unreadTotal(t, client); got != 0 {
		t.Fatalf("unread = %d, want 0", got)
	}
}

func TestMarkCategoryAsReadOlderThanDays(t *testing.T) {
	cfg := newIntegrationTestConfig()
	if !cfg.isConfigured() {
		t.Skip(skipIntegrationTestsMessage)
	}

	user, client, feedID := newMarkAsReadTestUser(t, cfg)
	feed, err := client.Feed(feedID)
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/v1/categories/%d/mark-all-as-read", feed.Category.ID)

	if status, _ := putMarkAllAsRead(t, cfg, user.Username, path, "-5"); status != http.StatusBadRequest {
		t.Fatalf("invalid value: status %d, want 400", status)
	}

	before := unreadTotal(t, client)
	status, body := putMarkAllAsRead(t, cfg, user.Username, path, "1")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
	if got := unreadTotal(t, client); int64(before-got) != body["changed_count"] {
		t.Fatalf("unread went %d -> %d but changed_count = %d", before, got, body["changed_count"])
	}

	if status, _ := putMarkAllAsRead(t, cfg, user.Username, path, ""); status != http.StatusNoContent {
		t.Fatalf("status %d, want 204", status)
	}
	if got := unreadTotal(t, client); got != 0 {
		t.Fatalf("unread = %d, want 0", got)
	}
}
