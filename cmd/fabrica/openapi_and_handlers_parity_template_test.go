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

	if !strings.Contains(got, "registerServicePaths(spec)") {
		t.Fatalf("openapi template should register service-level paths")
	}
	if !strings.Contains(got, "func registerServicePaths(spec *openapi3.T)") {
		t.Fatalf("openapi template should define registerServicePaths helper")
	}
	if !strings.Contains(got, "spec.Paths.Set(\"/health\"") {
		t.Fatalf("openapi template should include /health path")
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
}

func TestTemplate_HandlersUseSharedCRUDHelpers(t *testing.T) {
	common := mustReadFile(t, "pkg/codegen/templates/server/handlers_common.go.tmpl")
	if !strings.Contains(common, "func requireResourceUID(") {
		t.Fatalf("handlers common template should define requireResourceUID helper")
	}
	if !strings.Contains(common, "func decodeRequestJSON(") {
		t.Fatalf("handlers common template should define decodeRequestJSON helper")
	}
	if !strings.Contains(common, "func readRequestBody(") {
		t.Fatalf("handlers common template should define readRequestBody helper")
	}
	if !strings.Contains(common, "func publishNonCriticalEvent(") {
		t.Fatalf("handlers common template should define publishNonCriticalEvent helper")
	}
	if !strings.Contains(common, "func persistVersionSnapshot(") {
		t.Fatalf("handlers common template should define persistVersionSnapshot helper")
	}
	if !strings.Contains(common, "func applyStringMap(") {
		t.Fatalf("handlers common template should define applyStringMap helper")
	}
	if !strings.Contains(common, "type resourceAdapter[T any] struct") {
		t.Fatalf("handlers common template should define generic resourceAdapter core")
	}
	if !strings.Contains(common, "func createResourceWithAdapter[T any](") {
		t.Fatalf("handlers common template should define createResourceWithAdapter helper")
	}
	if !strings.Contains(common, "func mutateResourceWithAdapter[T any](") {
		t.Fatalf("handlers common template should define mutateResourceWithAdapter helper")
	}
	if !strings.Contains(common, "func mutateResourceWithAdapterNoHook[T any](") {
		t.Fatalf("handlers common template should define mutateResourceWithAdapterNoHook helper")
	}
	if !strings.Contains(common, "func deleteResourceWithAdapter[T any](") {
		t.Fatalf("handlers common template should define deleteResourceWithAdapter helper")
	}

	handlers := mustReadFile(t, "pkg/codegen/templates/server/handlers.go.tmpl")
	if !strings.Contains(handlers, "func {{camelCase .Name}}Identity(") {
		t.Fatalf("handlers template should define resource identity helper")
	}
	if !strings.Contains(handlers, "requireResourceUID(w, r, \"{{.Name}}\")") {
		t.Fatalf("handlers template should use requireResourceUID helper")
	}
	if !strings.Contains(handlers, "func {{camelCase .Name}}Adapter() resourceAdapter[") {
		t.Fatalf("handlers template should define per-resource adapter helper")
	}
	if !strings.Contains(handlers, "createResourceWithAdapter(w, r, adapter") {
		t.Fatalf("handlers template should use createResourceWithAdapter helper")
	}
	if !strings.Contains(handlers, "mutateResourceWithAdapter(w, r, adapter") {
		t.Fatalf("handlers template should use mutateResourceWithAdapter helper")
	}
	if !strings.Contains(handlers, "mutateResourceWithAdapterNoHook(w, r, adapter") {
		t.Fatalf("handlers template should use mutateResourceWithAdapterNoHook helper")
	}
	if !strings.Contains(handlers, "deleteResourceWithAdapter(w, r, adapter") {
		t.Fatalf("handlers template should use deleteResourceWithAdapter helper")
	}
	if !strings.Contains(handlers, "decodeRequestJSON(w, r") {
		t.Fatalf("handlers template should use decodeRequestJSON helper")
	}
	if !strings.Contains(handlers, "readRequestBody(w, r, \"patch data\")") {
		t.Fatalf("handlers template should use readRequestBody helper")
	}
	if !strings.Contains(handlers, "publishNonCriticalEvent(") {
		t.Fatalf("handlers template should use publishNonCriticalEvent helper")
	}
	if !strings.Contains(handlers, "{{camelCase .Name}}Identity(") {
		t.Fatalf("handlers template should use resource identity helper")
	}
	if !strings.Contains(handlers, "persistVersionSnapshot(") {
		t.Fatalf("handlers template should use persistVersionSnapshot helper")
	}
	if !strings.Contains(handlers, "applyStringMap(") {
		t.Fatalf("handlers template should use applyStringMap helper")
	}
}
