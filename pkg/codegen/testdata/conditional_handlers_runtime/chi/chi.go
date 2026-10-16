// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package chi

import (
	"net/http"
	"strings"
)

func URLParam(r *http.Request, key string) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if key == "uid" && len(parts) >= 2 {
		return parts[1]
	}
	if key == "versionID" && len(parts) >= 4 {
		return parts[3]
	}
	return ""
}
