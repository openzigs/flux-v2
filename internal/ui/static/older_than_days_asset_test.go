// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package static // import "miniflux.app/v2/internal/ui/static"

import (
	"os"
	"strings"
	"testing"
)

// The unread page offers an "older than N days" input next to "Mark all as read",
// and app.js sends it as older_than_days with the mark-all-as-read request.
func TestUnreadPageOlderThanDaysInput(t *testing.T) {
	tpl, err := os.ReadFile("../../template/templates/views/unread_entries.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`id="mark-all-older-than-days"`,
		`type="number"`,
		`min="1"`,
		`data-older-than-days-input="mark-all-older-than-days"`,
	} {
		if !strings.Contains(string(tpl), want) {
			t.Errorf("unread_entries.html is missing %s", want)
		}
	}

	js, err := os.ReadFile("js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"olderThanDaysInput", "older_than_days"} {
		if !strings.Contains(string(js), want) {
			t.Errorf("app.js is missing %s", want)
		}
	}
}
