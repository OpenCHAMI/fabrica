// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestParseResourceAnnotations_is_declaration_order_independent(t *testing.T) {
	// Given
	const resourceFirst = `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:sensitive
	Value string
}`
	const specFirst = `package v1

type TokenSpec struct {
	// +fabrica:field:sensitive
	Value string
}

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct { Spec TokenSpec }`
	tmpDir := t.TempDir()
	resourceFirstFile := filepath.Join(tmpDir, "resource_first.go")
	specFirstFile := filepath.Join(tmpDir, "spec_first.go")
	if err := os.WriteFile(resourceFirstFile, []byte(resourceFirst), 0o600); err != nil {
		t.Fatalf("Given resource-first source: %v", err)
	}
	if err := os.WriteFile(specFirstFile, []byte(specFirst), 0o600); err != nil {
		t.Fatalf("Given spec-first source: %v", err)
	}
	gen := NewGenerator(tmpDir, "test", "github.com/test/project")

	// When
	want, err := gen.ParseResourceAnnotations(resourceFirstFile, "Token")
	if err != nil {
		t.Fatalf("resource-first ParseResourceAnnotations() error = %v", err)
	}
	got, err := gen.ParseResourceAnnotations(specFirstFile, "Token")

	// Then
	if err != nil {
		t.Fatalf("spec-first ParseResourceAnnotations() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spec-first annotations = %#v, want %#v", got, want)
	}
}

func TestParseResourceAnnotations_propagates_annotation_parse_error(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "token.go")
	const source = `package v1

// +fabrica:resource
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:sensitve
	Value string
}`
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatalf("Given malformed source: %v", err)
	}
	gen := NewGenerator(tmpDir, "test", "github.com/test/project")

	// When
	got, err := gen.ParseResourceAnnotations(filename, "Token")

	// Then
	if err == nil {
		t.Fatalf("ParseResourceAnnotations() error = nil, result = %#v", got)
	}
	if got != nil {
		t.Fatalf("ParseResourceAnnotations() returned partial result %#v", got)
	}
	var parseErr *annotations.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("errors.As(*annotations.ParseError) = false: %v", err)
	}
	if !errors.Is(err, annotations.ErrAnnotationParse) {
		t.Fatalf("errors.Is(ErrAnnotationParse) = false: %v", err)
	}
	if parseErr.Filename != filename || parseErr.Line != 7 || parseErr.TypeName != "TokenSpec" || parseErr.FieldName != "Value" {
		t.Fatalf("ParseError source = %#v", parseErr)
	}
	if parseErr.Directive != "+fabrica:field:sensitve" || parseErr.Suggestion != "sensitive" || parseErr.Message == "" {
		t.Fatalf("ParseError diagnostic = %#v", parseErr)
	}
}

func TestParseResourceAnnotations_does_not_reuse_stale_source(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "token.go")
	const validSource = `package v1

// +fabrica:resource
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:sensitive
	Value string
}`
	if err := os.WriteFile(filename, []byte(validSource), 0o600); err != nil {
		t.Fatalf("Given valid source: %v", err)
	}
	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	if _, err := gen.ParseResourceAnnotations(filename, "Token"); err != nil {
		t.Fatalf("Given first parse: %v", err)
	}
	const changedSource = `package v1

// +fabrica:resource
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:sensitve
	Value string
}`
	if err := os.WriteFile(filename, []byte(changedSource), 0o600); err != nil {
		t.Fatalf("Given changed source: %v", err)
	}

	// When
	got, err := gen.ParseResourceAnnotations(filename, "Token")

	// Then
	if err == nil || got != nil {
		t.Fatalf("second parse = %#v, %v; want nil result and fresh parse error", got, err)
	}
	var parseErr *annotations.ParseError
	if !errors.As(err, &parseErr) || parseErr.Suggestion != "sensitive" {
		t.Fatalf("second parse error = %#v, %v", parseErr, err)
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
