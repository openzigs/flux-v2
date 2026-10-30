// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"log/slog"
	"net/http"
	"time"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
)

func (h *handler) markFeedAsRead(w http.ResponseWriter, r *http.Request) {
	feedID := request.RouteInt64Param(r, "feedID")
	userID := request.UserID(r)

	checkedAt, err := h.store.CheckedAt(userID, feedID)
	if err != nil {
		response.HTMLNotFound(w, r)
		return
	}

	before, err := olderThanCutoff(r, checkedAt)
	if err != nil {
		response.HTMLBadRequest(w, r, err)
		return
	}

	count, err := h.store.MarkFeedAsRead(userID, feedID, before)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	logMarkedAsRead("feed", userID, feedID, before, count)

	response.HTMLRedirect(w, r, h.routePath("/feeds"))
}

// olderThanCutoff narrows before to the older_than_days cutoff when the request carries one.
// It returns before unchanged when the parameter is absent, and an error when it is invalid.
func olderThanCutoff(r *http.Request, before time.Time) (time.Time, error) {
	days, hasAge, err := request.OlderThanDays(r)
	if err != nil || !hasAge {
		return before, err
	}

	cutoff := request.OlderThanDaysCutoff(time.Now(), days)
	if cutoff.Before(before) {
		return cutoff, nil
	}
	return before, nil
}

func logMarkedAsRead(scope string, userID, targetID int64, before time.Time, count int64) {
	slog.Debug("Marked entries as read from the web UI",
		slog.String("scope", scope),
		slog.Int64("user_id", userID),
		slog.Int64("target_id", targetID),
		slog.String("before", before.Format(time.RFC3339)),
		slog.Int64("nb_entries", count),
	)
}
