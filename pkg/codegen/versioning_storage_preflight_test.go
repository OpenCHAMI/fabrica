// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type storageVersioningDiagnostic interface {
	error
	Resource() string
	Configuration() string
	Backend() string
}

func TestPrepareResourceAnnotations_rejects_ent_version_snapshots_for_every_storage_mode(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "unannotated generic", source: unannotatedTokenSource},
		{name: "dedicated", source: dedicatedTokenSource},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			sourcePath := writeAnnotationSource(t, "token.go", versionedResourceSource(test.source))
			gen := NewGenerator(t.TempDir(), "main", "example.com/test")
			gen.SetStorageType("ent")
			gen.Resources = []ResourceMetadata{{Name: "Token", SourcePath: sourcePath}}

			// When
			err := gen.PrepareResourceAnnotations()

			// Then
			if err == nil {
				t.Fatal("PrepareResourceAnnotations() succeeded for Ent resource version snapshots")
			}
			var diagnostic storageVersioningDiagnostic
			if !errors.As(err, &diagnostic) {
				t.Fatalf("error %T does not expose typed storage-versioning context: %v", err, err)
			}
			if diagnostic.Resource() != "Token" || diagnostic.Configuration() != "+fabrica:resource-versioning=enabled" || diagnostic.Backend() != "ent" {
				t.Fatalf("diagnostic context = resource %q config %q backend %q", diagnostic.Resource(), diagnostic.Configuration(), diagnostic.Backend())
			}
		})
	}
}

func TestPrepareResourceAnnotations_rejects_tagged_ent_resource_without_annotation_source(t *testing.T) {
	// Given
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.SetStorageType("ent")
	gen.Resources = []ResourceMetadata{{
		Name: "Token",
		Tags: map[string]string{"versioning": "enabled"},
	}}

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if err == nil {
		t.Fatal("PrepareResourceAnnotations() succeeded for tagged Ent resource")
	}
}

func TestGenerateHandlers_rejects_ent_version_snapshots_before_rendering(t *testing.T) {
	// Given
	outputDir := t.TempDir()
	gen := NewGenerator(outputDir, "main", "example.com/test")
	gen.SetStorageType("ent")
	gen.Resources = []ResourceMetadata{{
		Name: "Token", PluralName: "tokens", StorageName: "Token",
		Tags: map[string]string{"versioning": "enabled"},
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}

	// When
	err := gen.GenerateHandlers()

	// Then
	if !errors.Is(err, ErrUnsupportedStorageFeature) {
		t.Fatalf("GenerateHandlers() error = %v, want unsupported storage feature", err)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "token_handlers_generated.go")); !os.IsNotExist(statErr) {
		t.Fatalf("handler output exists after preflight failure: %v", statErr)
	}
}

func versionedResourceSource(source string) string {
	return "// +fabrica:resource-versioning=enabled\n" + source
}
