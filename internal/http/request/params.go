// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request // import "miniflux.app/v2/internal/http/request"

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FormInt64Value returns the named form value parsed as int64, or 0 on error.
func FormInt64Value(r *http.Request, param string) int64 {
	value := r.FormValue(param)
	integer, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}

	return integer
}

// RouteInt64Param returns the named route parameter parsed as int64, or 0 when missing or invalid.
func RouteInt64Param(r *http.Request, param string) int64 {
	value, err := strconv.ParseInt(routeParam(r, param), 10, 64)
	if err != nil {
		return 0
	}

	if value < 0 {
		return 0
	}

	return value
}

// RouteStringParam returns the named route parameter as a string.
func RouteStringParam(r *http.Request, param string) string {
	return routeParam(r, param)
}

// QueryStringParam returns the named query parameter, or defaultValue if it is empty.
func QueryStringParam(r *http.Request, param, defaultValue string) string {
	value := r.URL.Query().Get(param)
	if value == "" {
		value = defaultValue
	}
	return value
}

// QueryStringParamList returns the non-empty, trimmed values for the named query parameter.
func QueryStringParamList(r *http.Request, param string) []string {
	var results []string
	values := r.URL.Query()

	if _, found := values[param]; found {
		for _, value := range values[param] {
			value = strings.TrimSpace(value)
			if value != "" {
				results = append(results, value)
			}
		}
	}

	return results
}

// QueryIntParam returns the named query parameter parsed as int, or defaultValue when missing, invalid, or negative.
func QueryIntParam(r *http.Request, param string, defaultValue int) int {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue
	}

	val, err := strconv.ParseInt(value, 10, 0)
	if err != nil {
		return defaultValue
	}

	if val < 0 {
		return defaultValue
	}

	return int(val)
}

// QueryInt64Param returns the named query parameter parsed as int64, or defaultValue when missing, invalid, or negative.
func QueryInt64Param(r *http.Request, param string, defaultValue int64) int64 {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue
	}

	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}

	if val < 0 {
		return defaultValue
	}

	return val
}

// QueryBoolParam returns the named query parameter parsed as bool, or defaultValue when missing or invalid.
func QueryBoolParam(r *http.Request, param string, defaultValue bool) bool {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue
	}

	val, err := strconv.ParseBool(value)

	if err != nil {
		return defaultValue
	}

	return val
}

// HasQueryParam reports whether the query string contains the named parameter.
func HasQueryParam(r *http.Request, param string) bool {
	values := r.URL.Query()
	_, ok := values[param]
	return ok
}

func routeParam(r *http.Request, param string) string {
	return r.PathValue(param)
}

// OlderThanDaysParam is the query parameter that selects the relative
// "older than N days" form of the mark-as-read actions. It is kept separate
// from any absolute "before" timestamp parameter.
const OlderThanDaysParam = "days"

// maxOlderThanDays bounds N so that N*24h cannot overflow a time.Duration.
const maxOlderThanDays = 36500

// ErrInvalidOlderThanDays is returned when the number of days is not a non-negative integer.
var ErrInvalidOlderThanDays = errors.New(`the "days" parameter must be a whole number of days between 0 and 36500`)

// ResolveOlderThanDaysCutoff validates a caller-supplied number of days and returns
// the absolute cutoff now - N*24h. Negative, non-numeric, empty or too large values are rejected.
func ResolveOlderThanDaysCutoff(value string, now time.Time) (time.Time, error) {
	days, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || days < 0 || days > maxOlderThanDays {
		return time.Time{}, ErrInvalidOlderThanDays
	}
	return now.Add(-time.Duration(days) * 24 * time.Hour), nil
}

// OlderThanDaysCutoff reads the optional "days" query parameter. present is false when the
// key is absent, in which case the caller keeps its existing no-cutoff behaviour. When the key
// is present, the value is validated and resolved by ResolveOlderThanDaysCutoff.
func OlderThanDaysCutoff(r *http.Request, now time.Time) (cutoff time.Time, present bool, err error) {
	if !HasQueryParam(r, OlderThanDaysParam) {
		return time.Time{}, false, nil
	}
	cutoff, err = ResolveOlderThanDaysCutoff(r.URL.Query().Get(OlderThanDaysParam), now)
	return cutoff, true, err
}
