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

func TestDedicatedStorageRouting_generated_helpers_have_authoritative_callers(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	operations := readGeneratedStorageFile(t, project.root, "storage_ent_resources_generated.go")
	queries := readGeneratedStorageFile(t, project.root, "ent_queries_generated.go")

	// Then
	for _, caller := range []string{"ToEntToken(", "FromEntToken(", "UpdateTokenFromResource(", "SaveTokenStatus(", "DeleteTokenByUID(", "ListTokens("} {
		if !strings.Contains(operations, caller) {
			t.Errorf("dedicated resource operations do not call %s", caller)
		}
	}
	if !strings.Contains(queries, "QueryTokenByName(") {
		t.Error("dedicated name helper has no generated caller")
	}
	if strings.Contains(operations, "entClient.Resource") || strings.Contains(operations, "ToEntResource(") {
		t.Error("dedicated resource operations retain a generic storage path")
	}
}

func TestDedicatedStorageRouting_regeneration_removes_stale_dedicated_artifacts(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runEntCodegenAndBuild(t, project, "dedicated")
	dedicatedEntityPath := filepath.Join(project.root, "internal", "storage", "ent", "token.go")
	if _, err := os.Stat(dedicatedEntityPath); err != nil {
		t.Fatalf("dedicated Ent entity was not generated: %v", err)
	}
	project.writeResourceSource(t, genericStorageTokenSource)

	// When
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runEntCodegenAndBuild(t, project, "generic")
	operations := readGeneratedStorageFile(t, project.root, "storage_ent_resources_generated.go")
	runtimePath := filepath.Join(project.root, "internal", "storage", "generic_mode_switch_runtime_test.go")
	if err := os.WriteFile(runtimePath, []byte(genericStorageRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generic mode-switch runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runtimeResult := project.run(t, "generic-mode-switch-runtime", project.root, "go", "test", "-count=1", "-run", "TestGenericStorage", "./internal/storage")
	if runtimeResult.err != nil {
		t.Fatalf("%s", runtimeResult.failureMessage())
	}

	// Then
	for _, stalePath := range []string{
		filepath.Join(project.root, "internal", "storage", "ent_adapter_token.go"),
		filepath.Join(project.root, "internal", "storage", "ent", "schema", "token.go"),
		dedicatedEntityPath,
		filepath.Join(project.root, "internal", "storage", "ent", "token_create.go"),
		filepath.Join(project.root, "internal", "storage", "ent", "token_delete.go"),
		filepath.Join(project.root, "internal", "storage", "ent", "token_query.go"),
		filepath.Join(project.root, "internal", "storage", "ent", "token_update.go"),
		filepath.Join(project.root, "internal", "storage", "ent", "token"),
	} {
		if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
			t.Errorf("stale dedicated artifact remains at %s: %v", stalePath, err)
		}
	}
	for name, forbidden := range map[string]string{
		"client.go":              "Token *TokenClient",
		"mutation.go":            "type TokenMutation struct",
		"predicate/predicate.go": "type Token func",
	} {
		content := readGeneratedEntFile(t, project.root, name)
		if strings.Contains(content, forbidden) {
			t.Errorf("regenerated Ent file %s retains %q", name, forbidden)
		}
	}
	if !strings.Contains(operations, "entClient.Resource") || strings.Contains(operations, "ToEntToken(") {
		t.Error("regenerated generic resource operations retained dedicated routing")
	}
}

func runEntCodegenAndBuild(t *testing.T, project generatedProject, mode string) {
	t.Helper()
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.run(t, mode+"-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.build(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
}

func readGeneratedStorageFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, "internal", "storage", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated storage file %s: %v", name, err)
	}
	return string(content)
}

func readGeneratedEntFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, "internal", "storage", "ent", filepath.FromSlash(name))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated Ent file %s: %v", name, err)
	}
	return string(content)
}
