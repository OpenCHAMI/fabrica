// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedFileStorage_CRUD_status_and_duplicate_values(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "file")
	project.writeResourceSource(t, genericStorageTokenSource)
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runtimePath := filepath.Join(project.root, "internal", "storage", "generated_file_acceptance_test.go")
	if err := os.WriteFile(runtimePath, []byte(generatedFileRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated file runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	result := project.run(t, "generated-file-storage-contract", project.root, "go", "test", "-count=1", "-v", "./internal/storage")

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if !strings.Contains(result.stdout, "file CRUD status duplicate-values") {
		t.Fatalf("runtime output missing file storage receipt\n%s", result.stdout)
	}
}

const generatedFileRuntimeTest = `package storage

import (
	"testing"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"github.com/openchami/fabrica/pkg/fabrica"
	fabricaStorage "github.com/openchami/fabrica/pkg/storage"
)

func TestGeneratedFileStorage_CRUD_status_and_duplicate_values(t *testing.T) {
	// Given
	backend, err := fabricaStorage.NewFileBackend(t.TempDir())
	if err != nil { t.Fatal(err) }
	Init(backend)
	first := &v1.Token{
		APIVersion: "acceptance.example.io/v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: "first", UID: "token-1"},
		Spec: v1.TokenSpec{Value: "shared"}, Status: v1.TokenStatus{State: "pending"},
	}

	// When
	if err := SaveToken(t.Context(), first); err != nil { t.Fatal(err) }
	first.Status.State = "ready"
	if err := SaveToken(t.Context(), first); err != nil { t.Fatal(err) }
	second := &v1.Token{
		APIVersion: first.APIVersion, Kind: first.Kind,
		Metadata: fabrica.Metadata{Name: "second", UID: "token-2"},
		Spec: v1.TokenSpec{Value: first.Spec.Value}, Status: v1.TokenStatus{State: "ready"},
	}
	if err := SaveToken(t.Context(), second); err != nil { t.Fatalf("duplicate value gained fake constraint behavior: %v", err) }
	loaded, err := LoadToken(t.Context(), first.Metadata.UID)
	if err != nil { t.Fatal(err) }
	all, err := LoadAllTokens(t.Context())
	if err != nil { t.Fatal(err) }
	if err := DeleteToken(t.Context(), first.Metadata.UID); err != nil { t.Fatal(err) }

	// Then
	if loaded.Status.State != "ready" || loaded.Spec.Value != "shared" { t.Fatalf("loaded=%#v", loaded) }
	if len(all) != 2 { t.Fatalf("list length=%d, want 2", len(all)) }
	t.Log("file CRUD status duplicate-values")
}
`
