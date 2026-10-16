// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"
	"time"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
)

func (h *handler) markAllAsRead(w http.ResponseWriter, r *http.Request) {
	// With no age chosen, before stays zero and every globally visible unread entry is marked.
	before, _, err := request.OlderThanDaysCutoff(r, time.Now())
	if err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	if err := h.store.MarkGloballyVisibleFeedsAsRead(request.UserID(r), before); err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	response.JSON(w, r, "OK")
}
