// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"net/http"
	"testing"
)

func TestMarkAllAsReadOlderThanDays(t *testing.T) {
	client := NewClientWithOptions(
		"http://mf",
		WithHTTPClient(
			newFakeHTTPClient(t, func(t *testing.T, req *http.Request) *http.Response {
				expectRequest(t, http.MethodPut, "http://mf/v1/users/1/mark-all-as-read?older_than_days=30", nil, req)
				return jsonResponseFrom(t, http.StatusOK, http.Header{}, map[string]int64{"changed_count": 12})
			})))
	count, err := client.MarkAllAsReadOlderThanDaysContext(t.Context(), 1, 30)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if count != 12 {
		t.Fatalf("Expected changed count 12, got %d", count)
	}
}

func TestMarkFeedAsReadOlderThanDays(t *testing.T) {
	client := NewClientWithOptions(
		"http://mf",
		WithHTTPClient(
			newFakeHTTPClient(t, func(t *testing.T, req *http.Request) *http.Response {
				expectRequest(t, http.MethodPut, "http://mf/v1/feeds/7/mark-all-as-read?older_than_days=14", nil, req)
				return jsonResponseFrom(t, http.StatusOK, http.Header{}, map[string]int64{"changed_count": 3})
			})))
	count, err := client.MarkFeedAsReadOlderThanDaysContext(t.Context(), 7, 14)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if count != 3 {
		t.Fatalf("Expected changed count 3, got %d", count)
	}
}

func TestMarkAsReadOlderThanDaysRejectsNonPositiveDays(t *testing.T) {
	client := NewClientWithOptions(
		"http://mf",
		WithHTTPClient(
			newFakeHTTPClient(t, func(t *testing.T, req *http.Request) *http.Response {
				t.Fatalf("no request expected, got %s", req.URL)
				return nil
			})))
	if _, err := client.MarkAllAsReadOlderThanDaysContext(t.Context(), 1, 0); err == nil {
		t.Fatal("Expected an error for 0 days")
	}
	if _, err := client.MarkFeedAsReadOlderThanDaysContext(t.Context(), 1, -5); err == nil {
		t.Fatal("Expected an error for -5 days")
	}
}
