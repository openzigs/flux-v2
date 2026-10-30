// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"
	"time"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
)

func (h *handler) markCategoryAsRead(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	categoryID := request.RouteInt64Param(r, "categoryID")

	category, err := h.store.Category(userID, categoryID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}

	if category == nil {
		response.HTMLNotFound(w, r)
		return
	}

	before, err := olderThanCutoff(r, time.Now())
	if err != nil {
		response.HTMLBadRequest(w, r, err)
		return
	}

	count, err := h.store.MarkCategoryAsRead(userID, categoryID, before)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	logMarkedAsRead("category", userID, categoryID, before, count)

	response.HTMLRedirect(w, r, h.routePath("/categories"))
}
