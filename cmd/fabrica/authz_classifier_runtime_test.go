// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
)

// DefaultClassifyRequestForAuthZ is a minimal copy of the generated default classifier.
//
// It avoids chi as a dependency of Fabrica itself; tests that validate RoutePattern
// behavior are done separately by checking template contents.
func DefaultClassifyRequestForAuthZ(r *http.Request) (subject, object, action string, protected, ok bool, reason string) {
	protected = true

	subject = ""
	action = r.Method
	if action == "" {
		return subject, "", "", protected, false, "missing HTTP method"
	}

	object = r.URL.Path
	reason = "object derived from URL path"
	if object == "" {
		return subject, "", action, protected, false, "missing object"
	}

	if subject == "" {
		return subject, object, action, protected, false, "missing subject"
	}

	return subject, object, action, protected, true, reason
}
