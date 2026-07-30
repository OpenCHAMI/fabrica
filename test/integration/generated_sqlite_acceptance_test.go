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

func TestGeneratedSQLite_acceptance(t *testing.T) {
	t.Run("generic resource baseline", TestGeneratedAnnotationProject_generic_storage_CRUD_and_queries_remain_compatible)
	t.Run("dedicated schema constraints defaults and CRUD", testGeneratedSQLiteDedicatedContract)
	t.Run("dedicated bcrypt and redaction", TestDedicatedSecurity_generated_adapter_runtime)
	t.Run("complete envelope survives reopen", TestGeneratedDedicatedAdapter_roundtrips_complete_envelope_through_sqlite_reload)
	t.Run("mixed resources route exclusively", TestMixedStorage_generated_runtime_routes_each_resource_exclusively)
	t.Run("explicit migration preview rerun and rollback", TestDedicatedMigration_generated_SQLite_runtime)
	t.Run("SQLite rejects PostgreSQL GIN before dedicated output", testGeneratedSQLiteRejectsGIN)
}

func testGeneratedSQLiteDedicatedContract(t *testing.T) {
	t.Helper()
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, generatedSQLiteTokenSource)
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.run(t, "sqlite-acceptance-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runtimePath := filepath.Join(project.root, "internal", "storage", "generated_sqlite_acceptance_test.go")
	if err := os.WriteFile(runtimePath, []byte(generatedSQLiteRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated SQLite runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	result := project.run(t, "generated-sqlite-dedicated-contract", project.root, "go", "test", "-count=1", "-v", "-run", "TestGeneratedSQLite", "./internal/storage")
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	t.Logf("dedicated SQLite receipts:\n%s", result.stdout)
	for _, receipt := range []string{"schema DDL", "CRUD zero immutable unique", "dedicated create conflict chain", "dedicated update conflict chain", "reopen envelope corrupt status"} {
		if !strings.Contains(result.stdout, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, result.stdout)
		}
	}
}

func testGeneratedSQLiteRejectsGIN(t *testing.T) {
	t.Helper()
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, generatedSQLiteGINTokenSource)
	sourcePath := filepath.Join(project.root, "apis", "acceptance.example.io", "v1", "token_types.go")
	before, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read GIN source: %v", err)
	}

	result := project.generate(t)
	if result.err == nil || !strings.Contains(strings.ToLower(result.stderr+result.stdout), "gin") {
		t.Fatalf("SQLite GIN generation error = %v\nstdout=%s\nstderr=%s", result.err, result.stdout, result.stderr)
	}
	after, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("reread GIN source: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("failed SQLite GIN generation modified the resource source")
	}
	generatedSchema := filepath.Join(project.root, "internal", "storage", "ent", "schema", "token.go")
	if _, err := os.Stat(generatedSchema); !os.IsNotExist(err) {
		t.Fatalf("SQLite GIN rejection left dedicated schema output: %v", err)
	}
	t.Logf("SQLite GIN rejected before output: %s", strings.TrimSpace(result.stderr+result.stdout))
}
