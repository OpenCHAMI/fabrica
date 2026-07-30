// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestRegisterResource_without_source_remains_generic(t *testing.T) {
	// Given
	type DeviceSpec struct {
		Name string
	}
	type Device struct {
		Spec DeviceSpec
	}
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")

	// When
	err := gen.RegisterResource(&Device{})

	// Then
	if err != nil {
		t.Fatalf("RegisterResource() error = %v", err)
	}
	if len(gen.Resources) != 1 {
		t.Fatalf("registered resources = %d, want 1", len(gen.Resources))
	}
	if gen.Resources[0].Annotations != nil {
		t.Fatalf("programmatic resource annotations = %#v, want nil generic selection", gen.Resources[0].Annotations)
	}
}

func TestPrepareResourceAnnotations_attaches_complete_source_metadata(t *testing.T) {
	// Given
	type TokenSpec struct {
		Value string
	}
	type Token struct {
		Spec TokenSpec
	}
	sourcePath := writeAnnotationSource(t, "token.go", dedicatedTokenSource)
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.SetStorageType("ent")
	if err := gen.RegisterResourceFromSource(&Token{}, sourcePath); err != nil {
		t.Fatalf("RegisterResourceFromSource() error = %v", err)
	}

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if err != nil {
		t.Fatalf("PrepareResourceAnnotations() error = %v", err)
	}
	resource := gen.Resources[0]
	if resource.SourcePath != sourcePath {
		t.Fatalf("SourcePath = %q, want %q", resource.SourcePath, sourcePath)
	}
	if resource.Annotations == nil || resource.Annotations.StorageMode != annotations.StorageModeDedicated {
		t.Fatalf("annotations = %#v, want dedicated", resource.Annotations)
	}
	if !resource.Annotations.Fields["Value"].Sensitive {
		t.Fatalf("Value annotations = %#v, want sensitive", resource.Annotations.Fields["Value"])
	}
	if got := strings.Join(resource.Annotations.RawAnnotations, "\n"); !strings.Contains(got, "+fabrica:storage=dedicated") {
		t.Fatalf("raw annotations = %q, want storage directive", got)
	}
}

func TestPrepareResourceAnnotations_handles_multiple_mixed_resources(t *testing.T) {
	// Given
	type TokenSpec struct{ Value string }
	type Token struct{ Spec TokenSpec }
	type DeviceSpec struct{ Name string }
	type Device struct{ Spec DeviceSpec }
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.SetStorageType("ent")
	if err := gen.RegisterResourceFromSource(
		&Token{},
		writeAnnotationSource(t, "token.go", dedicatedTokenSource),
	); err != nil {
		t.Fatalf("register Token: %v", err)
	}
	if err := gen.RegisterResourceFromSource(
		&Device{},
		writeAnnotationSource(t, "device.go", unannotatedDeviceSource),
	); err != nil {
		t.Fatalf("register Device: %v", err)
	}

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if err != nil {
		t.Fatalf("PrepareResourceAnnotations() error = %v", err)
	}
	if got := gen.Resources[0].Annotations.StorageMode; got != annotations.StorageModeDedicated {
		t.Fatalf("Token storage mode = %q, want dedicated", got)
	}
	if got := gen.Resources[1].Annotations.StorageMode; got != annotations.StorageModeGeneric {
		t.Fatalf("Device storage mode = %q, want generic", got)
	}
}

func TestPrepareResourceAnnotations_attaches_by_registration_identity_when_names_collide(t *testing.T) {
	// Given
	firstPath := writeAnnotationSource(t, "first/token.go", dedicatedTokenSource)
	secondPath := writeAnnotationSource(t, "second/token.go", unannotatedTokenSource)
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.SetStorageType("ent")
	gen.Resources = []ResourceMetadata{
		{Name: "Token", Package: "example.com/first", SourcePath: firstPath},
		{Name: "Token", Package: "example.com/second", SourcePath: secondPath},
	}

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if err != nil {
		t.Fatalf("PrepareResourceAnnotations() error = %v", err)
	}
	if got := gen.Resources[0].Annotations.StorageMode; got != annotations.StorageModeDedicated {
		t.Fatalf("first Token storage mode = %q, want dedicated", got)
	}
	if got := gen.Resources[1].Annotations.StorageMode; got != annotations.StorageModeGeneric {
		t.Fatalf("second Token storage mode = %q, want generic", got)
	}
}

func TestPrepareResourceAnnotations_joins_source_errors_without_partial_attachment(t *testing.T) {
	// Given
	firstPath := writeAnnotationSource(t, "first.go", malformedTokenSource)
	secondPath := writeAnnotationSource(t, "second.go", strings.ReplaceAll(malformedTokenSource, "Token", "Device"))
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.Resources = []ResourceMetadata{
		{Name: "Token", SourcePath: firstPath},
		{Name: "Device", SourcePath: secondPath},
	}

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if err == nil {
		t.Fatal("PrepareResourceAnnotations() error = nil, want joined source errors")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 2 {
		t.Fatalf("joined errors = %#v, want two independent failures", err)
	}
	for _, expected := range []string{firstPath, secondPath, "sensitve"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("joined error missing %q: %v", expected, err)
		}
	}
	var parseErr *annotations.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("errors.As(*annotations.ParseError) = false: %v", err)
	}
	for _, resource := range gen.Resources {
		if resource.Annotations != nil {
			t.Errorf("resource %s received partial annotations %#v", resource.Name, resource.Annotations)
		}
	}
}

func writeAnnotationSource(t *testing.T, name, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write annotation source: %v", err)
	}
	return path
}

const dedicatedTokenSource = `package fixture

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:sensitive
	Value string
}`

const unannotatedTokenSource = `package fixture

type Token struct { Spec TokenSpec }
type TokenSpec struct { Value string }`

const unannotatedDeviceSource = `package fixture

type Device struct { Spec DeviceSpec }
type DeviceSpec struct { Name string }`

const malformedTokenSource = `package fixture

// +fabrica:resource
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:sensitve
	Value string
}`
