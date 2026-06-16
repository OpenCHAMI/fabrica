// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_InitMain_AuthNRequiresJWKSURL(t *testing.T) {
	authHelperTmpl := mustReadFile(t, "pkg/codegen/templates/init/auth_helpers.go.tmpl")

	if !strings.Contains(authHelperTmpl, "TOKENSMITH_JWKS_URL is required") {
		t.Fatalf("init/auth_helpers.go.tmpl must hard-fail startup when TOKENSMITH_JWKS_URL is missing")
	}
	if !strings.Contains(authHelperTmpl, "os.Getenv(\"TOKENSMITH_JWKS_URL\")") {
		t.Fatalf("init/auth_helpers.go.tmpl must read TOKENSMITH_JWKS_URL")
	}
	if !strings.Contains(authHelperTmpl, "{{.TokenSmithModulePath}}/pkg/authn") {
		t.Fatalf("init/auth_helpers.go.tmpl must import TokenSmith pkg/authn for new generated services")
	}
	if !strings.Contains(authHelperTmpl, "tokensmithauthn.Middleware(tokensmithauthn.Options{") {
		t.Fatalf("init/auth_helpers.go.tmpl must use the TokenSmith authn middleware API")
	}
	if strings.Contains(authHelperTmpl, "tokensmith/middleware") {
		t.Fatalf("init/auth_helpers.go.tmpl must not reference the legacy TokenSmith middleware submodule")
	}
	if strings.Contains(authHelperTmpl, "NewJWTAuthMiddleware") {
		t.Fatalf("init/auth_helpers.go.tmpl must not reference the removed root-package TokenSmith JWT helper")
	}
}
