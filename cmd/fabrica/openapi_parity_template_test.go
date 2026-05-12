// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_OpenAPIIncludesServiceHealthPath(t *testing.T) {
	got := mustReadFile(t, "pkg/codegen/templates/server/openapi.go.tmpl")

	if !strings.Contains(got, "healthOp := openapi3.NewOperation()") {
		t.Fatalf("openapi template should define /health operation")
	}
	if !strings.Contains(got, "healthOp.OperationID = \"health\"") {
		t.Fatalf("openapi template should set health operation ID")
	}
	if !strings.Contains(got, "spec.Paths.Set(\"/health\"") {
		t.Fatalf("openapi template should include /health path")
	}
	if !strings.Contains(got, "healthResponseSchema := openapi3.NewObjectSchema()") {
		t.Fatalf("openapi template should define health response schema")
	}
	if !strings.Contains(got, "healthExample := map[string]interface{}") {
		t.Fatalf("openapi template should define health response example payload")
	}
	if !strings.Contains(got, "Example: healthExample") {
		t.Fatalf("openapi template should set media type example for /health response")
	}
}

func TestTemplate_OpenAPIIncludesPatchAndStatusSubresourcePaths(t *testing.T) {
	got := mustReadFile(t, "pkg/codegen/templates/server/openapi.go.tmpl")

	if !strings.Contains(got, "Patch:      patchOp") {
		t.Fatalf("resource item path should include PATCH operation")
	}
	if !strings.Contains(got, "spec.Paths.Set(\"{{.URLPath}}/{uid}/status\", statusPath)") {
		t.Fatalf("openapi template should include status subresource path")
	}
	if !strings.Contains(got, "update{{.Name}}Status") {
		t.Fatalf("openapi template should include update status operation ID")
	}
	if !strings.Contains(got, "patch{{.Name}}Status") {
		t.Fatalf("openapi template should include patch status operation ID")
	}
	if !strings.Contains(got, "func patchRequestBody() *openapi3.RequestBodyRef") {
		t.Fatalf("openapi template should define patch request body helper")
	}
	if !strings.Contains(got, "application/merge-patch+json") {
		t.Fatalf("openapi template should include application/merge-patch+json patch content type")
	}
	if !strings.Contains(got, "application/json-patch+json") {
		t.Fatalf("openapi template should include application/json-patch+json patch content type")
	}
	if !strings.Contains(got, "application/shorthand-patch") {
		t.Fatalf("openapi template should include application/shorthand-patch patch content type")
	}
}
