// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package api // import "miniflux.app/v2/internal/api"

import (
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
