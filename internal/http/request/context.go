// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request // import "miniflux.app/v2/internal/http/request"

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"miniflux.app/v2/internal/model"
)

// ContextKey represents a context key.
type ContextKey int

// List of context keys.
const (
	UserIDContextKey ContextKey = iota
	UserNameContextKey
	UserTimezoneContextKey
	IsAdminUserContextKey
	IsAuthenticatedContextKey
	WebSessionContextKey
	ClientIPContextKey
	GoogleReaderTokenKey
)

// WebSession returns the current web session from the request context, if present.
func WebSession(r *http.Request) *model.WebSession {
	if v := r.Context().Value(WebSessionContextKey); v != nil {
		if value, valid := v.(*model.WebSession); valid {
			return value
		}
	}
	return nil
}

// GoogleReaderToken returns the Google Reader token from the request context, if present.
func GoogleReaderToken(r *http.Request) string {
	return getContextStringValue(r, GoogleReaderTokenKey)
}

// IsAdminUser reports whether the logged-in user is an administrator.
func IsAdminUser(r *http.Request) bool {
	return getContextBoolValue(r, IsAdminUserContextKey)
}

// IsAuthenticated reports whether the user is authenticated.
func IsAuthenticated(r *http.Request) bool {
	if getContextBoolValue(r, IsAuthenticatedContextKey) {
		return true
	}

	if session := WebSession(r); session != nil {
		return session.IsAuthenticated()
	}

	return false
}

// UserID returns the logged-in user's ID from the request context.
func UserID(r *http.Request) int64 {
	if userID := getContextInt64Value(r, UserIDContextKey); userID != 0 {
		return userID
	}

	if session := WebSession(r); session != nil {
		if id, ok := session.UserID(); ok {
			return id
		}
	}

	return 0
}

// UserName returns the logged-in user's username, or "unknown" when unset.
func UserName(r *http.Request) string {
	value := getContextStringValue(r, UserNameContextKey)
	if value == "" {
		value = "unknown"
	}
	return value
}

// UserTimezone returns the user's timezone, defaulting to "UTC" when unset.
func UserTimezone(r *http.Request) string {
	value := getContextStringValue(r, UserTimezoneContextKey)
	if value == "" {
		value = "UTC"
	}
	return value
}

// ClientIP returns the client IP address stored in the request context.
func ClientIP(r *http.Request) string {
	return getContextStringValue(r, ClientIPContextKey)
}

func getContextStringValue(r *http.Request, key ContextKey) string {
	if v := r.Context().Value(key); v != nil {
		if value, valid := v.(string); valid {
			return value
		}
	}
	return ""
}

func getContextBoolValue(r *http.Request, key ContextKey) bool {
	if v := r.Context().Value(key); v != nil {
		if value, valid := v.(bool); valid {
			return value
		}
	}
	return false
}

func getContextInt64Value(r *http.Request, key ContextKey) int64 {
	if v := r.Context().Value(key); v != nil {
		if value, valid := v.(int64); valid {
			return value
		}
	}
	return 0
}

// OlderThanDaysParam is the query or form parameter carrying a relative age in days.
const OlderThanDaysParam = "older_than_days"

// MaxOlderThanDays bounds the relative age accepted by OlderThanDays (about 100 years).
const MaxOlderThanDays = 36500

// ErrInvalidOlderThanDays is returned when older_than_days is present but not a positive integer.
var ErrInvalidOlderThanDays = errors.New("older_than_days must be a positive integer of days")

// OlderThanDays returns the validated older_than_days value from the query string or form.
// present is false when the parameter is absent or empty; err is set when it is present but
// not an integer between 1 and MaxOlderThanDays.
func OlderThanDays(r *http.Request) (days int, present bool, err error) {
	value := r.FormValue(OlderThanDaysParam)
	if value == "" {
		return 0, false, nil
	}

	parsed, parseErr := strconv.Atoi(value)
	if parseErr != nil || parsed < 1 || parsed > MaxOlderThanDays {
		return 0, true, ErrInvalidOlderThanDays
	}

	return parsed, true, nil
}

// OlderThanDaysCutoff returns the instant N days before now; entries published strictly before it qualify.
func OlderThanDaysCutoff(now time.Time, days int) time.Time {
	return now.AddDate(0, 0, -days)
}
