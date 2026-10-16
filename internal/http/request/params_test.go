// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request // import "miniflux.app/v2/internal/http/request"

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestFormInt64Value(t *testing.T) {
	f := url.Values{}
	f.Set("integer value", "42")
	f.Set("invalid value", "invalid integer")

	r := &http.Request{Form: f}

	result := FormInt64Value(r, "integer value")
	expected := int64(42)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = FormInt64Value(r, "invalid value")
	expected = int64(0)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = FormInt64Value(r, "missing value")
	expected = int64(0)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}
}

func TestRouteStringParamWithServerMux(t *testing.T) {
	router := http.NewServeMux()
	router.HandleFunc("GET /route/{variable}/index", func(w http.ResponseWriter, r *http.Request) {
		result := RouteStringParam(r, "variable")
		expected := "value"

		if result != expected {
			t.Errorf(`Unexpected result, got %q instead of %q`, result, expected)
		}

		result = RouteStringParam(r, "missing variable")
		expected = ""

		if result != expected {
			t.Errorf(`Unexpected result, got %q instead of %q`, result, expected)
		}
	})

	r, err := http.NewRequest(http.MethodGet, "/route/value/index", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
}

func TestRouteInt64ParamWithServerMux(t *testing.T) {
	router := http.NewServeMux()
	router.HandleFunc("GET /a/{variable1}/b/{variable2}/c/{variable3}", func(w http.ResponseWriter, r *http.Request) {
		result := RouteInt64Param(r, "variable1")
		expected := int64(42)

		if result != expected {
			t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
		}

		result = RouteInt64Param(r, "missing variable")
		expected = 0

		if result != expected {
			t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
		}

		result = RouteInt64Param(r, "variable2")
		expected = 0

		if result != expected {
			t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
		}

		result = RouteInt64Param(r, "variable3")
		expected = 0

		if result != expected {
			t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
		}
	})

	r, err := http.NewRequest(http.MethodGet, "/a/42/b/not-int/c/-10", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
}

func TestQueryStringParam(t *testing.T) {
	u, _ := url.Parse("http://example.org/?key=value")
	r := &http.Request{URL: u}

	result := QueryStringParam(r, "key", "fallback")
	expected := "value"

	if result != expected {
		t.Errorf(`Unexpected result, got %q instead of %q`, result, expected)
	}

	result = QueryStringParam(r, "missing key", "fallback")
	expected = "fallback"

	if result != expected {
		t.Errorf(`Unexpected result, got %q instead of %q`, result, expected)
	}
}

func TestQueryIntParam(t *testing.T) {
	u, _ := url.Parse("http://example.org/?key=42&invalid=value&negative=-5")
	r := &http.Request{URL: u}

	result := QueryIntParam(r, "key", 84)
	expected := 42

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = QueryIntParam(r, "missing key", 84)
	expected = 84

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = QueryIntParam(r, "negative", 69)
	expected = 69

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = QueryIntParam(r, "invalid", 99)
	expected = 99

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}
}

func TestQueryInt64Param(t *testing.T) {
	u, _ := url.Parse("http://example.org/?key=42&invalid=value&negative=-5")
	r := &http.Request{URL: u}

	result := QueryInt64Param(r, "key", int64(84))
	expected := int64(42)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = QueryInt64Param(r, "missing key", int64(84))
	expected = int64(84)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = QueryInt64Param(r, "negative", int64(69))
	expected = int64(69)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}

	result = QueryInt64Param(r, "invalid", int64(99))
	expected = int64(99)

	if result != expected {
		t.Errorf(`Unexpected result, got %d instead of %d`, result, expected)
	}
}

func TestQueryBoolParam(t *testing.T) {
	u, _ := url.Parse("http://example.org/?truthy=true&falsy=false&invalid=wat")
	r := &http.Request{URL: u}

	result := QueryBoolParam(r, "truthy", false)
	expected := true

	if result != expected {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}

	result = QueryBoolParam(r, "falsy", true)
	expected = false

	if result != expected {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}

	result = QueryBoolParam(r, "missing", true)
	expected = true

	if result != expected {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}

	result = QueryBoolParam(r, "invalid", true)
	expected = true

	if result != expected {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}
}

func TestQueryStringParamList(t *testing.T) {
	u, _ := url.Parse("http://example.org/?tag=alpha&tag=beta&tag=+&tag=%20%20gamma%20%20&empty=")
	r := &http.Request{URL: u}

	result := QueryStringParamList(r, "tag")
	expected := []string{"alpha", "beta", "gamma"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}

	result = QueryStringParamList(r, "missing")
	expected = nil

	if !reflect.DeepEqual(result, expected) {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}
}

func TestHasQueryParam(t *testing.T) {
	u, _ := url.Parse("http://example.org/?key=42")
	r := &http.Request{URL: u}

	result := HasQueryParam(r, "key")
	expected := true

	if result != expected {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}

	result = HasQueryParam(r, "missing key")
	expected = false

	if result != expected {
		t.Errorf(`Unexpected result, got %v instead of %v`, result, expected)
	}
}

func TestResolveOlderThanDaysCutoff(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		value   string
		want    time.Time
		wantErr bool
	}{
		{name: "one day is exactly 24 hours", value: "1", want: now.Add(-24 * time.Hour)},
		{name: "thirty days", value: "30", want: now.Add(-30 * 24 * time.Hour)},
		{name: "zero days is now", value: "0", want: now},
		{name: "surrounding spaces", value: " 7 ", want: now.Add(-7 * 24 * time.Hour)},
		{name: "negative", value: "-5", wantErr: true},
		{name: "non numeric", value: "abc", wantErr: true},
		{name: "decimal", value: "1.5", wantErr: true},
		{name: "empty", value: "", wantErr: true},
		{name: "too large", value: "100000", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveOlderThanDaysCutoff(tc.value, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got cutoff %v", tc.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.value, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("cutoff for %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestOlderThanDaysCutoffFromQuery(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		query       string
		wantPresent bool
		want        time.Time
		wantErr     bool
	}{
		{name: "absent key", query: "", wantPresent: false},
		{name: "other keys only", query: "before=123", wantPresent: false},
		{name: "valid", query: "days=30", wantPresent: true, want: now.Add(-30 * 24 * time.Hour)},
		{name: "empty value", query: "days=", wantPresent: true, wantErr: true},
		{name: "negative", query: "days=-1", wantPresent: true, wantErr: true},
		{name: "non numeric", query: "days=abc", wantPresent: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/?"+tc.query, nil)
			got, present, err := OlderThanDaysCutoff(r, now)
			if present != tc.wantPresent {
				t.Fatalf("present = %v, want %v", present, tc.wantPresent)
			}
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantPresent && !tc.wantErr && !got.Equal(tc.want) {
				t.Errorf("cutoff = %v, want %v", got, tc.want)
			}
		})
	}
}
