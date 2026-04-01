// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestInitGoMod_TokenSmithDependencyConditional(t *testing.T) {
	tmpl := mustReadFile(t, "pkg/codegen/templates/init/go.mod.tmpl")

	t.Run("security_off_no_tokensmith", func(t *testing.T) {
		out := mustRenderInitTemplate(t, tmpl, templateData{WithAuth: false})
		if strings.Contains(out, "tokensmith") {
			t.Fatalf("expected no tokensmith in rendered go.mod, got:\n%s", out)
		}
	})

	t.Run("security_on_tokensmith_pinned", func(t *testing.T) {
		out := mustRenderInitTemplate(t, tmpl, templateData{WithAuth: true, TokenSmithVersion: "v0.0.1"})
		if !strings.Contains(out, "github.com/openchami/tokensmith v0.0.1") {
			t.Fatalf("expected tokensmith pinned dependency, got:\n%s", out)
		}
	})
}
