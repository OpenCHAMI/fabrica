// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type generatedProjectExpectations struct {
	dedicatedResources []string
	genericResources   []string
}

func (p generatedProject) setDBDriver(t *testing.T, driver string) {
	t.Helper()
	path := filepath.Join(p.root, ".fabrica.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated-project configuration: %v", err)
	}
	if strings.Contains(string(content), "db_driver: "+driver) {
		return
	}
	updated := strings.Replace(string(content), "db_driver: sqlite", "db_driver: "+driver, 1)
	if updated == string(content) {
		t.Fatalf("generated-project configuration has no SQLite driver to replace")
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write generated-project configuration: %v", err)
	}
}

func (p generatedProject) requireGeneratedProjectGate(t *testing.T, expected generatedProjectExpectations) {
	t.Helper()
	stages := []func(*testing.T) commandResult{
		p.generate,
		p.tidy,
		func(t *testing.T) commandResult {
			return p.run(t, "ent-codegen", p.root, "go", "generate", "./internal/storage")
		},
		p.tidy,
	}
	for _, stage := range stages {
		if result := stage(t); result.err != nil {
			t.Fatalf("%s", result.failureMessage())
		}
	}
	if err := verifyPinnedEntRequirement(p.root); err != nil {
		t.Fatalf("stage %q failed: %v", "go-mod-ent-pin", err)
	}
	moduleResult := p.run(t, "go-list-ent-module", p.root, "go", "list", "-m", "-f", "{{.Path}} {{.Version}}", generatedEntModule)
	if moduleResult.err != nil {
		t.Fatalf("%s", moduleResult.failureMessage())
	}
	if got, want := strings.TrimSpace(moduleResult.stdout), generatedEntModule+" "+generatedEntVersion; got != want {
		t.Fatalf("stage %q resolved %q, want %q\n--- stderr ---\n%s", moduleResult.stage, got, want, moduleResult.stderr)
	}
	t.Logf("stage %q resolved module: %s", moduleResult.stage, strings.TrimSpace(moduleResult.stdout))
	if err := verifyGeneratedProjectArtifacts(p.root, expected); err != nil {
		t.Fatalf("stage %q failed: %v", "generated-artifact-contract", err)
	}
	for _, stage := range []func(*testing.T) commandResult{p.vet, p.build} {
		if result := stage(t); result.err != nil {
			t.Fatalf("%s", result.failureMessage())
		}
	}
}

func verifyGeneratedProjectArtifacts(root string, expected generatedProjectExpectations) error {
	for _, resource := range expected.dedicatedResources {
		lower := strings.ToLower(resource)
		for _, path := range []string{
			filepath.Join("internal", "storage", "ent", "schema", lower+".go"),
			filepath.Join("internal", "storage", "ent", lower+".go"),
			filepath.Join("internal", "storage", "ent", lower, "where.go"),
			filepath.Join("internal", "storage", "ent", lower+"_update.go"),
			filepath.Join("internal", "storage", "ent_adapter_"+lower+".go"),
		} {
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				return fmt.Errorf("dedicated %s artifact %s: %w", resource, path, err)
			}
		}
	}
	for _, resource := range expected.genericResources {
		path := filepath.Join(root, "internal", "storage", "ent", "schema", strings.ToLower(resource)+".go")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return fmt.Errorf("generic %s unexpectedly has dedicated schema %s", resource, path)
		}
	}
	return verifyDedicatedStorageCallers(root, expected.dedicatedResources)
}
