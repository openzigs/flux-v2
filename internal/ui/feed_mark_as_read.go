// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
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

	cutoff, hasDays, err := request.OlderThanDaysCutoff(r, time.Now())
	if err != nil {
		response.HTMLBadRequest(w, r, err)
		return
	}
	if !hasDays {
		cutoff = checkedAt
	}

	if _, err = h.store.MarkFeedAsRead(userID, feedID, cutoff); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}

	response.HTMLRedirect(w, r, h.routePath("/feeds"))
}
