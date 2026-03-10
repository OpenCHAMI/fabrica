// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_DefaultAuthZClassifier_HasRoutePatternFallbackLogic(t *testing.T) {
	got := mustReadTemplate(t, "server/authz_classifier.go.tmpl")

	// Ensure we prefer chi.RouteContext.RoutePattern() and fall back to URL path.
	if !strings.Contains(got, "chi.RouteContext") || !strings.Contains(got, "RoutePattern()") {
		t.Fatalf("default classifier template missing chi RoutePattern usage")
	}
	if !strings.Contains(got, "r.URL.Path") {
		t.Fatalf("default classifier template missing URL path fallback")
	}
	if !strings.Contains(got, "object derived from chi route pattern") {
		t.Fatalf("default classifier template missing route pattern reason string")
	}
	if !strings.Contains(got, "object derived from URL path") {
		t.Fatalf("default classifier template missing URL path reason string")
	}
}
