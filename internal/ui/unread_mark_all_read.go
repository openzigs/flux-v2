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

func (h *handler) markAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)

	days, hasAge, err := request.OlderThanDays(r)
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	// The zero cutoff keeps the date-unconstrained behaviour when no age is given.
	var cutoff time.Time
	if hasAge {
		cutoff = request.OlderThanDaysCutoff(time.Now(), days)
	}

	count, err := h.store.MarkGloballyVisibleFeedsAsRead(userID, cutoff)
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if !hasAge {
		response.JSON(w, r, "OK")
		return
	}

	slog.Info("Marked globally visible entries older than N days as read",
		slog.Int64("user_id", userID),
		slog.Int("older_than_days", days),
		slog.String("cutoff", cutoff.Format(time.RFC3339)),
		slog.Int64("nb_entries", count),
	)
	response.JSON(w, r, map[string]int64{"changed_count": count})
}
