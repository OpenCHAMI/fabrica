// Copyright © 2026 OpenCHAMI Contributors
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

type ProgrammaticTokenSpec struct {
	Value string `json:"value" validate:"required"`
	Name  string `json:"display-name" validate:"required"`
}

type ProgrammaticToken struct {
	Spec ProgrammaticTokenSpec
}

func TestGenerateEntDedicated_programmatic_registration_without_source_generates_schema_and_adapter(t *testing.T) {
	// Given
	root := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change to fixture directory: %v", err)
	}

	gen := NewGenerator(root, "test", "example.com/programmatic")
	gen.StorageType = "ent"
	gen.DBDriver = "sqlite"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	if err := gen.RegisterResource(&ProgrammaticToken{}); err != nil {
		t.Fatalf("register reflected resource: %v", err)
	}
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	value := annotations.NewFieldAnnotations("Value")
	value.Immutable = true
	annots.Fields["Value"] = value
	gen.SetResourceAnnotations("ProgrammaticToken", annots)

	// When
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("generate dedicated schema: %v", err)
	}
	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("generate dedicated adapter: %v", err)
	}

	// Then
	schema, err := os.ReadFile(filepath.Join(root, "internal", "storage", "ent", "schema", "programmatictoken.go"))
	if err != nil {
		t.Fatalf("read generated schema: %v", err)
	}
	adapter, err := os.ReadFile(filepath.Join(root, "internal", "storage", "ent_adapter_programmatictoken.go"))
	if err != nil {
		t.Fatalf("read generated adapter: %v", err)
	}
	if !strings.Contains(string(schema), `field.String("spec_display_name")`) {
		t.Fatalf("schema did not use reflected JSON field mapping\n%s", schema)
	}
	updateBody := generatedSection(t, string(adapter), "func UpdateProgrammaticTokenFromResource", "func QueryProgrammaticTokenByName")
	if strings.Contains(updateBody, "SetSpecValue(resource.Spec.Value)") {
		t.Fatalf("immutable field has an update setter\n%s", adapter)
	}
}

func generatedSection(t *testing.T, source, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("generated source missing start marker %q", startMarker)
	}
	end := strings.Index(source[start:], endMarker)
	if end < 0 {
		t.Fatalf("generated source missing end marker %q", endMarker)
	}
	return source[start : start+end]
}
