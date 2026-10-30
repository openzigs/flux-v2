// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request // import "miniflux.app/v2/internal/http/request"

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOlderThanDaysQuery(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		wantDays    int
		wantPresent bool
		wantErr     bool
	}{
		{"absent", "", 0, false, false},
		{"empty value", "older_than_days=", 0, false, false},
		{"valid", "older_than_days=30", 30, true, false},
		{"zero", "older_than_days=0", 0, true, true},
		{"negative", "older_than_days=-5", 0, true, true},
		{"not a number", "older_than_days=abc", 0, true, true},
		{"too large", "older_than_days=1000000", 0, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodPut, "http://example.org/v1/users/1/mark-all-as-read?"+tc.query, nil)
			days, present, err := OlderThanDays(r)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if present != tc.wantPresent {
				t.Fatalf("present = %v, want %v", present, tc.wantPresent)
			}
			if days != tc.wantDays {
				t.Fatalf("days = %d, want %d", days, tc.wantDays)
			}
		})
	}
}

func TestOlderThanDaysForm(t *testing.T) {
	form := url.Values{"older_than_days": {"7"}}
	r, _ := http.NewRequest(http.MethodPost, "http://example.org/mark-all-as-read", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	days, present, err := OlderThanDays(r)
	if err != nil || !present || days != 7 {
		t.Fatalf("got (%d, %v, %v), want (7, true, nil)", days, present, err)
	}
}
