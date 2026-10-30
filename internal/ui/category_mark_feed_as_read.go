// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"

	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
)

func (h *handler) markCategoryFeedAsRead(w http.ResponseWriter, r *http.Request) {
	feedID := request.RouteInt64Param(r, "feedID")
	categoryID := request.RouteInt64Param(r, "categoryID")
	userID := request.UserID(r)

	if !h.store.CategoryFeedExists(userID, categoryID, feedID) {
		response.HTMLNotFound(w, r)
		return
	}

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
	logMarkedAsRead("category_feed", userID, feedID, before, count)

	response.HTMLRedirect(w, r, h.routePath("/category/%d/feeds", categoryID))
}
