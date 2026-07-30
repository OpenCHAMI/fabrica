// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestHandlerHookTemplate_emits_fields_and_calls_for_enabled_operations(t *testing.T) {
	resource := operationTestResource("Widget", annotations.OperationPolicy{
		List: true, Get: true, Create: true, Update: true, Patch: true, Delete: true,
		StatusUpdate: true, StatusPatch: true, VersionList: true, VersionGet: true,
		VersionDelete: true, Exposure: annotations.ExposureProtected,
	})
	gen := NewGenerator(t.TempDir(), "main", "example.com/service")
	gen.Resources = []ResourceMetadata{resource}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	handlers := renderOperationTemplate(t, gen, "handlers", gen.templateData(resource, "server/handlers.go.tmpl"))

	assertContainsAll(t, handlers,
		"type WidgetHooks struct",
		"BeforeList", "AfterList",
		"BeforeGet", "AfterGet",
		"BeforeCreate", "ExecuteCreate", "AfterCreate",
		"BeforeUpdate", "ExecuteUpdate", "AfterUpdate",
		"BeforePatch", "ExecutePatch", "AfterPatch",
		"BeforeDelete", "ExecuteDelete", "AfterDelete",
		"BeforeStatusUpdate", "ExecuteStatusUpdate", "AfterStatusUpdate",
		"BeforeStatusPatch", "ExecuteStatusPatch", "AfterStatusPatch",
		"BeforeVersionList", "AfterVersionList",
		"BeforeVersionGet", "AfterVersionGet",
		"BeforeVersionDelete", "AfterVersionDelete",
		"widgetHooks.BeforeCreate", "widgetHooks.ExecuteCreate", "widgetHooks.AfterCreate",
	)
	assertContainsNone(t, handlers, "ExecuteList", "ExecuteGet", "ExecuteVersionList", "ExecuteVersionGet", "ExecuteVersionDelete")
}

func TestHandlerHookTemplate_omits_hooks_for_private_and_disabled_resources(t *testing.T) {
	tests := []struct {
		name   string
		policy annotations.OperationPolicy
	}{
		{name: "private with explicit operation", policy: annotations.OperationPolicy{List: true, Exposure: annotations.ExposurePrivate}},
		{name: "verbs none", policy: annotations.OperationPolicy{Exposure: annotations.ExposureProtected}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := operationTestResource("Widget", tt.policy)
			gen := NewGenerator(t.TempDir(), "main", "example.com/service")
			gen.Resources = []ResourceMetadata{resource}
			if err := gen.LoadTemplates(); err != nil {
				t.Fatalf("LoadTemplates() error = %v", err)
			}

			handlers := renderOperationTemplate(t, gen, "handlers", gen.templateData(resource, "server/handlers.go.tmpl"))

			assertContainsAll(t, handlers, "type WidgetHooks struct")
			assertContainsNone(t, handlers, "BeforeList", "AfterList", "widgetHooks.")
		})
	}
}

func TestHandlerHookGeneration_preserves_create_once_file(t *testing.T) {
	outputDir := t.TempDir()
	resource := operationTestResource("Widget", annotations.OperationPolicy{Create: true, Exposure: annotations.ExposureProtected})
	gen := NewGenerator(outputDir, "main", "example.com/service")
	gen.Resources = []ResourceMetadata{resource}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}
	if err := gen.GenerateHandlers(); err != nil {
		t.Fatalf("GenerateHandlers() error = %v", err)
	}
	hookPath := filepath.Join(outputDir, "widget_hooks.go")
	generated, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read generated hook file: %v", err)
	}
	if !strings.Contains(string(generated), "var widgetHooks = WidgetHooks{}") {
		t.Fatalf("generated hook file missing stable variable declaration:\n%s", generated)
	}

	custom := []byte("package main\n\n// user customization\nvar widgetHooks = WidgetHooks{}\n")
	if err := os.WriteFile(hookPath, custom, 0o644); err != nil {
		t.Fatalf("customize hook file: %v", err)
	}
	if err := gen.GenerateHandlers(); err != nil {
		t.Fatalf("regenerate handlers: %v", err)
	}
	preserved, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read preserved hook file: %v", err)
	}
	if string(preserved) != string(custom) {
		t.Fatalf("create-once hook file changed:\n%s", preserved)
	}
}

func TestHandlerHookGeneration_omits_create_once_file_when_hooks_disabled(t *testing.T) {
	tests := []struct {
		name   string
		policy annotations.OperationPolicy
	}{
		{name: "private", policy: annotations.OperationPolicy{List: true, Exposure: annotations.ExposurePrivate}},
		{name: "none", policy: annotations.OperationPolicy{Exposure: annotations.ExposureProtected}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := t.TempDir()
			resource := operationTestResource("Widget", tt.policy)
			gen := NewGenerator(outputDir, "main", "example.com/service")
			gen.Resources = []ResourceMetadata{resource}
			if err := gen.LoadTemplates(); err != nil {
				t.Fatalf("LoadTemplates() error = %v", err)
			}
			if err := gen.GenerateHandlers(); err != nil {
				t.Fatalf("GenerateHandlers() error = %v", err)
			}
			_, err := os.Stat(filepath.Join(outputDir, "widget_hooks.go"))
			if !os.IsNotExist(err) {
				t.Fatalf("disabled hooks generated create-once file: %v", err)
			}
		})
	}
}

func TestGeneratedHandlerError_renders_safe_mapping_contract(t *testing.T) {
	resource := operationTestResource("Widget", annotations.OperationPolicy{Create: true, Exposure: annotations.ExposureProtected})
	gen := NewGenerator(t.TempDir(), "main", "example.com/service")
	gen.Resources = []ResourceMetadata{resource}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() error = %v", err)
	}

	models := renderOperationTemplate(t, gen, "models", gen.globalTemplateData("server/models.go.tmpl"))

	assertContainsAll(t, models,
		"type HandlerError struct",
		"StatusCode",
		"PublicMessage string",
		"Cause",
		"func (e *HandlerError) Unwrap() error",
		"func respondHookError(",
		"errors.As(err, &handlerErr)",
		"status < http.StatusBadRequest || status > 599",
	)
}
