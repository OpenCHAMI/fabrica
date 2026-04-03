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
	if !strings.Contains(mainTmpl, "{{.TokenSmithModulePath}}/pkg/authn") {
		t.Fatalf("init/main.go.tmpl must import TokenSmith pkg/authn for new generated services")
	}
	if !strings.Contains(mainTmpl, "tokensmithauthn.Middleware(tokensmithauthn.Options{") {
		t.Fatalf("init/main.go.tmpl must use the TokenSmith authn middleware API")
	}
	if strings.Contains(mainTmpl, "tokensmith/middleware") {
		t.Fatalf("init/main.go.tmpl must not reference the legacy TokenSmith middleware submodule")
	}
	if strings.Contains(mainTmpl, "NewJWTAuthMiddleware") {
		t.Fatalf("init/main.go.tmpl must not reference the removed root-package TokenSmith JWT helper")
	}
}
