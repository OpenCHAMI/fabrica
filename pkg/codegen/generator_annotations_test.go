// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestParseResourceAnnotations(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "token_types.go")
	source := `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	Value string ` + "`json:\"value\"`" + `

	// +fabrica:field:index
	Name string ` + "`json:\"name\"`" + `
}
`

	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	gen.DBDriver = "postgres"

	annots, err := gen.ParseResourceAnnotations(testFile, "Token")
	if err != nil {
		t.Fatalf("ParseResourceAnnotations failed: %v", err)
	}

	if !annots.IsResource {
		t.Error("expected IsResource=true")
	}

	if annots.StorageMode != annotations.StorageModeDedicated {
		t.Errorf("expected StorageMode=dedicated, got %s", annots.StorageMode)
	}

	if len(annots.Fields) != 2 {
		t.Errorf("expected 2 field annotations, got %d", len(annots.Fields))
	}

	valueField, ok := annots.Fields["Value"]
	if !ok {
		t.Fatal("expected annotation for Value field")
	}

	if !valueField.Sensitive {
		t.Error("expected Value field to be sensitive")
	}

	if valueField.Storage == nil {
		t.Fatal("expected Storage config for Value field")
	}

	if valueField.Storage.Type != annotations.StorageTypeHashed {
		t.Errorf("expected hashed storage, got %s", valueField.Storage.Type)
	}

	nameField, ok := annots.Fields["Name"]
	if !ok {
		t.Fatal("expected annotation for Name field")
	}

	if nameField.Index == nil {
		t.Fatal("expected Index config for Name field")
	}
}

func TestSetResourceAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "test", "github.com/test/project")

	type TokenSpec struct {
		Value string
	}

	type Token struct {
		Spec TokenSpec
	}

	if err := gen.RegisterResource(&Token{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	gen.SetResourceAnnotations("Token", annots)

	if len(gen.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(gen.Resources))
	}

	resource := gen.Resources[0]
	if resource.Annotations == nil {
		t.Fatal("expected annotations to be set")
	}

	if resource.Annotations.StorageMode != annotations.StorageModeDedicated {
		t.Errorf("expected dedicated storage mode, got %s", resource.Annotations.StorageMode)
	}
}

func TestParseResourceAnnotationsNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	source := `package v1

type Other struct {
	Name string
}
`

	if err := os.WriteFile(testFile, []byte(source), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")

	annots, err := gen.ParseResourceAnnotations(testFile, "Token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if annots.IsResource {
		t.Error("expected IsResource=false for non-existent resource")
	}
}
