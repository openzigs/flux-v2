// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/locale"
)

// maxMarkAsReadAgeInDays bounds N so that N*24h cannot overflow time.Duration.
const maxMarkAsReadAgeInDays = 36500

var errInvalidMarkAsReadAge = errors.New("the number of days must be a whole number between 1 and 36500")

func (h *handler) markAllAsRead(w http.ResponseWriter, r *http.Request) {
	if err := h.store.MarkGloballyVisibleFeedsAsRead(request.UserID(r)); err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.JSON(w, r, "OK")
}

// markOlderThanAsRead marks the user's unread entries published more than N days ago as read.
func (h *handler) markOlderThanAsRead(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	sess := request.WebSession(r)
	printer := locale.NewPrinter(sess.Language())

	days, err := parseAgeInDays(r.FormValue("days"))
	if err != nil {
		sess.SetErrorMessage(printer.Print("error.invalid_mark_as_read_age"))
		response.HTMLRedirect(w, r, h.routePath("/unread"))
		return
	}

	now := time.Now()
	cutoff := ageCutoff(now, days)
	count, err := h.store.MarkAllAsReadBeforeDate(userID, cutoff)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}

	slog.Debug("Marked entries older than N days as read from the web ui",
		slog.Int64("user_id", userID),
		slog.Int("days", days),
		slog.Int64("nb_entries", count),
	)

	sess.SetSuccessMessage(printer.Printf("alert.entries_marked_as_read", count))
	response.HTMLRedirect(w, r, h.routePath("/unread"))
}

// parseAgeInDays parses a strictly positive whole number of days.
func parseAgeInDays(value string) (int, error) {
	days, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || days < 1 || days > maxMarkAsReadAgeInDays {
		return 0, errInvalidMarkAsReadAge
	}
	return days, nil
}

// ageCutoff returns the instant exactly n fixed 24-hour days before now.
func ageCutoff(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days) * 24 * time.Hour)
}
