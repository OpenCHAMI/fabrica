// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_InitMain_AuthNRequiresJWKSURL(t *testing.T) {
	mainTmpl := mustReadFile(t, "pkg/codegen/templates/init/main.go.tmpl")

	if !strings.Contains(mainTmpl, "TOKENSMITH_JWKS_URL is required") {
		t.Fatalf("init/main.go.tmpl must hard-fail startup when TOKENSMITH_JWKS_URL is missing")
	}
	if !strings.Contains(mainTmpl, "os.Getenv(\"TOKENSMITH_JWKS_URL\")") {
		t.Fatalf("init/main.go.tmpl must read TOKENSMITH_JWKS_URL")
	}
}
