// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedFileVersioning_builds_and_runs_snapshot_runtime(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "file")
	source := strings.Replace(genericStorageTokenSource, "type TokenStatus struct {\n\tState string", "type TokenStatus struct {\n\tVersion string `json:\"version,omitempty\"`\n\tState string", 1)
	project.writeResourceSource(t, versionedResourceSource(source))
	mainPath := filepath.Join(project.root, "cmd", "server", "main.go")
	if err := os.WriteFile(mainPath, []byte("package main\n\ntype Config struct{}\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write complete project main fixture: %v", err)
	}
	result := project.run(t, "generate-file-versioning", project.root, project.fabricaBin, "generate", "--force", "--debug", "--fabrica-source", project.repoRoot)
	if result.err == nil {
		result = project.tidy(t)
	}
	if result.err == nil {
		result = project.build(t)
	}
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runtimePath := filepath.Join(project.root, "internal", "storage", "versioning_runtime_test.go")
	if err := os.WriteFile(runtimePath, []byte(generatedFileVersioningRuntimeTest), 0o644); err != nil {
		t.Fatalf("write snapshot runtime test: %v", err)
	}

	// When
	result = project.run(t, "file-versioning-runtime", project.root, "go", "test", "-count=1", "-run", "TestVersionSnapshotRuntime", "./internal/storage")

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
}

func TestGeneratedEntWithoutVersioning_builds_for_generic_and_dedicated_storage(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		generate bool
	}{
		{name: "generic", source: generatedUnannotatedTokenSource, generate: true},
		{name: "dedicated", source: validAnnotatedTokenSource},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, "ent")
			project.writeResourceSource(t, test.source)

			// When
			result := project.generate(t)
			if result.err == nil {
				result = project.tidy(t)
			}
			if result.err == nil && test.generate {
				result = project.run(t, "generic-ent-codegen", project.root, "go", "generate", "./internal/storage")
			}
			if result.err == nil {
				result = project.build(t)
			}

			// Then
			if result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}
		})
	}
}

const generatedFileVersioningRuntimeTest = `package storage

import (
	"testing"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"github.com/openchami/fabrica/pkg/fabrica"
)

func TestVersionSnapshotRuntime(t *testing.T) {
	// Given
	t.Chdir(t.TempDir())
	if err := InitFileBackend("./data"); err != nil { t.Fatal(err) }
	token := &v1.Token{
		APIVersion: "acceptance.example.io/v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: "first", UID: "token-1"},
		Spec: v1.TokenSpec{Value: "secret"},
	}

	// When
	versionID, err := CreateTokenVersionSnapshot(t.Context(), token)
	if err != nil { t.Fatal(err) }
	snapshot, err := GetTokenVersion(t.Context(), token.Metadata.UID, versionID)
	if err != nil { t.Fatal(err) }
	latest, err := LatestTokenVersionID(t.Context(), token.Metadata.UID)
	if err != nil { t.Fatal(err) }

	// Then
	if snapshot.Spec.Value != token.Spec.Value || snapshot.UID != token.Metadata.UID { t.Fatalf("snapshot=%#v", snapshot) }
	if latest != versionID { t.Fatalf("latest=%q, want %q", latest, versionID) }
}
`
