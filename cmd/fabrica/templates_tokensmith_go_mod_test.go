// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"

	"github.com/openchami/fabrica/internal/constants"
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
		out := mustRenderInitTemplate(t, tmpl, templateData{WithAuth: true, GoVersion: constants.TokenSmithGoVersion, TokenSmithModulePath: constants.TokenSmithModulePath, TokenSmithVersion: "v9.9.9"})
		if !strings.Contains(out, constants.TokenSmithModulePath+" v9.9.9") {
			t.Fatalf("expected tokensmith pinned dependency, got:\n%s", out)
		}
		if !strings.Contains(out, "go "+constants.TokenSmithGoVersion) {
			t.Fatalf("expected TokenSmith-auth projects to use go %s, got:\n%s", constants.TokenSmithGoVersion, out)
		}
	})
}
