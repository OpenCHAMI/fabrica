// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

//go:build integration

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedPostgres_acceptance(t *testing.T) {
	requiredPostgresDSN(t)
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, generatedPostgresTokenSource())
	writePostgresGeneratedProjectConfig(t, project)

	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.run(t, "postgres-acceptance-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runtimePath := filepath.Join(project.root, "internal", "storage", "generated_postgres_acceptance_test.go")
	fixture := generatedPostgresRuntimeHelpers + generatedPostgresRuntimeContract + generatedPostgresRuntimeCatalog
	if err := os.WriteFile(runtimePath, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write generated PostgreSQL runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	result := project.run(t, "generated-postgres-contract", project.root, "go", "test", "-count=1", "-v", "-run", "^TestGeneratedPostgresRuntime", "./internal/storage")
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	t.Logf("generated PostgreSQL receipts:\n%s", result.stdout)
	for _, receipt := range []string{
		"role catalog",
		"DDL defaults nullability indexes",
		"CRUD unique immutable bcrypt redaction",
		"full envelope reopen mixed routing",
		"redacted status hidden preservation",
		"explicit migration preview copy rerun",
	} {
		if !strings.Contains(result.stdout, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, result.stdout)
		}
	}
}

func writePostgresGeneratedProjectConfig(t *testing.T, project generatedProject) {
	t.Helper()
	configPath := filepath.Join(project.root, ".fabrica.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read generated project config: %v", err)
	}
	updated := strings.Replace(string(config), "db_driver: sqlite", "db_driver: postgres", 1)
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write PostgreSQL project config: %v", err)
	}
	widgetPath := filepath.Join(project.root, "apis", "acceptance.example.io", "v1", "widget_types.go")
	if err := os.WriteFile(widgetPath, []byte(mixedGenericWidgetSource), 0o644); err != nil {
		t.Fatalf("write generic widget source: %v", err)
	}
	apis := "groups:\n  - name: acceptance.example.io\n    storageVersion: v1\n    versions: [v1]\n    resources: [Token, Widget]\n"
	if err := os.WriteFile(filepath.Join(project.root, "apis.yaml"), []byte(apis), 0o644); err != nil {
		t.Fatalf("write mixed PostgreSQL API config: %v", err)
	}
}
