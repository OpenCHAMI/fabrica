// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_RouterGrouping_PublicAndProtected(t *testing.T) {
	got := mustReadTemplate(t, "server/routes.go.tmpl")

	// Public docs/openapi routes live in the init main template so they remain
	// outside auth middleware.
	if strings.Contains(got, "public.Get(\"/openapi.json\"") {
		t.Fatalf("routes template should not register /openapi.json directly")
	}
	if strings.Contains(got, "public.Get(\"/docs\"") {
		t.Fatalf("routes template should not register /docs directly")
	}

	// Protected group should exist and resource routes should be under it.
	if !strings.Contains(got, "// Protected endpoints (resource APIs)") {
		t.Fatalf("routes template missing protected endpoints grouping comment")
	}
	if !strings.Contains(got, "r.Group(func(protected chi.Router)") {
		t.Fatalf("routes template missing protected chi group")
	}
	if !strings.Contains(got, "protected.Route(\"{{.URLPath}}\"") {
		t.Fatalf("resource routes should be mounted using protected.Route")
	}

	// Ensure we did not accidentally duplicate or add global OPTIONS handling.
	if strings.Contains(got, "Options(") || strings.Contains(got, "Method(\"OPTIONS\"") {
		t.Fatalf("routes template unexpectedly contains explicit OPTIONS handlers")
	}
}

func TestTemplate_MainRouterHasPublicProtectedGroups(t *testing.T) {
	got := mustReadFile(t, "pkg/codegen/templates/init/main.go.tmpl")
	if !strings.Contains(got, "public.Get(\"/health\"") {
		t.Fatalf("main template should register /health on the public group")
	}
	if !strings.Contains(got, "public.Get(\"/openapi.json\"") {
		t.Fatalf("main template should register /openapi.json on the public group")
	}
	if !strings.Contains(got, "public.Get(\"/docs\"") {
		t.Fatalf("main template should register /docs on the public group")
	}
	if !strings.Contains(got, "protected.Use(authnMiddleware)") {
		t.Fatalf("main template should apply authn middleware on protected routes")
	}
}
