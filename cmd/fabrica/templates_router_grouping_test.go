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

	// Public routes should be explicitly grouped.
	if !strings.Contains(got, "// Public endpoints (bypass AuthN/AuthZ)") {
		t.Fatalf("routes template missing public endpoints grouping comment")
	}
	if !strings.Contains(got, "r.Group(func(public chi.Router)") {
		t.Fatalf("routes template missing public chi group")
	}
	if !strings.Contains(got, "public.Get(\"/openapi.json\"") {
		t.Fatalf("routes template should register /openapi.json on public group")
	}
	if !strings.Contains(got, "public.Get(\"/docs\"") {
		t.Fatalf("routes template should register /docs on public group")
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
	// NOTE: the init server "main" template is not loaded into
	// codegen.Generator.LoadTemplates() today (see TestDumpTemplateKeys).
	// We validate public/protected routing via server/routes.go.tmpl.
	t.Skip("init/main.go.tmpl is not loaded into codegen.Generator templates")
}
