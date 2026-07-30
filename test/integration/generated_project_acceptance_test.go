// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedProjectBaseline_discovers_resource_and_writes_registration(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "file")
	project.writeResourceSource(t, generatedUnannotatedTokenSource)

	// When
	result := project.generate(t)

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if !strings.Contains(result.stdout, "Found 1 resource(s): Token") {
		t.Fatalf(
			"stage %q exited successfully without discovery receipt\n--- stdout ---\n%s\n--- stderr ---\n%s",
			result.stage,
			result.stdout,
			result.stderr,
		)
	}
	registrationPath := filepath.Join(project.root, "pkg", "resources", "register_generated.go")
	if _, err := os.Stat(registrationPath); err != nil {
		t.Fatalf("expected registration artifact %s after successful generation: %v", registrationPath, err)
	}
}

func TestGeneratedAnnotationProject_attaches_source_annotations(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")

	// When
	result := project.generate(t)

	// Then
	for _, artifact := range []string{
		filepath.Join("internal", "storage", "ent", "schema", "token.go"),
		filepath.Join("internal", "storage", "ent_adapter_token.go"),
	} {
		path := filepath.Join(project.root, artifact)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf(
				"stage %q exited successfully without dedicated artifact %s: %v\n--- stdout ---\n%s\n--- stderr ---\n%s",
				result.stage,
				artifact,
				err,
				result.stdout,
				result.stderr,
			)
		}
	}
}

func TestGeneratedAnnotationProject_builds_dedicated_ent_storage(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")

	// When
	generateResult := project.generate(t)
	if generateResult.err != nil {
		t.Fatalf("%s", generateResult.failureMessage())
	}
	tidyResult := project.tidy(t)
	if tidyResult.err != nil {
		t.Fatalf("%s", tidyResult.failureMessage())
	}
	buildResult := project.build(t)

	// Then
	if buildResult.err != nil {
		t.Fatalf("%s", buildResult.failureMessage())
	}
	dedicatedSchema := filepath.Join(project.root, "internal", "storage", "ent", "schema", "token.go")
	if _, err := os.Stat(dedicatedSchema); err != nil {
		t.Fatalf(
			"stage %q exited successfully but dedicated schema is absent: %v\n--- generate stdout ---\n%s\n--- generate stderr ---\n%s\n--- build stdout ---\n%s\n--- build stderr ---\n%s",
			buildResult.stage,
			err,
			generateResult.stdout,
			generateResult.stderr,
			buildResult.stdout,
			buildResult.stderr,
		)
	}
}

func TestGeneratedAnnotationProject_builds_complete_storage_backend_matrix(t *testing.T) {
	for _, storageType := range []string{"file", "ent"} {
		t.Run(storageType, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, storageType)
			project.writeResourceSource(t, generatedUnannotatedTokenSource)
			mainPath := filepath.Join(project.root, "cmd", "server", "main.go")
			if err := os.WriteFile(mainPath, []byte("package main\n\ntype Config struct{}\n\nfunc main() {}\n"), 0o644); err != nil {
				t.Fatalf("write complete project main fixture: %v", err)
			}

			// When
			generateResult := project.run(
				t,
				"fabrica-generate-complete-project",
				project.root,
				project.fabricaBin,
				"generate",
				"--force",
				"--debug",
				"--fabrica-source",
				project.repoRoot,
			)
			if generateResult.err != nil {
				t.Fatalf("%s", generateResult.failureMessage())
			}
			if result := project.tidy(t); result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}
			buildResult := project.build(t)

			// Then
			if buildResult.err != nil {
				t.Fatalf("%s", buildResult.failureMessage())
			}
			conflictContract := filepath.Join(project.root, "internal", "storage", "errors_generated.go")
			if _, err := os.Stat(conflictContract); err != nil {
				t.Fatalf("storage backend %q omitted shared conflict contract: %v", storageType, err)
			}
		})
	}
}

func TestGeneratedAnnotationProject_rejects_malformed_directive_before_ent_codegen(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "misspelled sensitive field directive",
			source: malformedAnnotatedTokenSource,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, "ent")
			project.writeResourceSource(t, test.source)

			// When
			result := project.generate(t)

			// Then
			if result.err == nil {
				t.Fatalf(
					"stage %q succeeded for malformed directive; expected source-located TokenSpec.Value failure before Ent codegen\n--- stdout ---\n%s\n--- stderr ---\n%s",
					result.stage,
					result.stdout,
					result.stderr,
				)
			}
			combined := result.stdout + result.stderr
			for _, contextPart := range []string{"token_types.go", "TokenSpec", "Value", "sensitve"} {
				if !strings.Contains(combined, contextPart) {
					t.Errorf("malformed directive failure missing %q context\n%s", contextPart, result.failureMessage())
				}
			}
			entSchemaDir := filepath.Join(project.root, "internal", "storage", "ent", "schema")
			if _, err := os.Stat(entSchemaDir); !os.IsNotExist(err) {
				t.Errorf("malformed directive reached Ent generation; schema directory stat error: %v", err)
			}
		})
	}
}

func TestGeneratedAnnotationProject_does_not_reuse_stale_registration(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "file")
	project.writeResourceSource(t, generatedUnannotatedTokenSource)
	firstResult := project.generate(t)
	if firstResult.err != nil {
		t.Fatalf("%s", firstResult.failureMessage())
	}
	updatedSource := strings.NewReplacer(
		"Token", "Credential",
		"token", "credential",
	).Replace(generatedUnannotatedTokenSource)
	project.writeResourceSource(t, updatedSource)

	// When
	secondResult := project.generate(t)

	// Then
	if secondResult.err != nil {
		t.Fatalf("%s", secondResult.failureMessage())
	}
	registrationPath := filepath.Join(project.root, "pkg", "resources", "register_generated.go")
	registration, err := os.ReadFile(registrationPath)
	if err != nil {
		t.Fatalf("read regenerated registration: %v", err)
	}
	if strings.Contains(string(registration), "Token") || !strings.Contains(string(registration), "Credential") {
		t.Fatalf(
			"successive generation reused stale registration\n--- registration ---\n%s\n--- stdout ---\n%s\n--- stderr ---\n%s",
			registration,
			secondResult.stdout,
			secondResult.stderr,
		)
	}
}
