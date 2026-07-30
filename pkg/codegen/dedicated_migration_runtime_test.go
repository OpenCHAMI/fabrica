// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDedicatedMigration_generated_SQLite_runtime(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, dedicatedMigrationTokenSource)
	if result := project.generate(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.build(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if result := project.vet(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	checkGeneratedMigrationWithGopls(t, project)
	if result := project.run(t, "migration-ent-codegen", project.root, "go", "generate", "./internal/storage"); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	path := filepath.Join(project.root, "internal", "storage", "dedicated_migration_runtime_test.go")
	fixture := dedicatedMigrationRuntimeHelpers + dedicatedMigrationRuntimeTests + dedicatedMigrationFailureRuntimeTests + dedicatedMigrationCancellationRuntimeTests
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write dedicated migration runtime fixture: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	// When
	result := project.run(t, "dedicated-migration-sqlite-runtime", project.root, "go", "test", "-count=1", "-v", "-run", "TestDedicatedMigration", "./internal/storage")

	// Then
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	t.Logf("generated migration runtime output:\n%s", result.stdout)
	for _, receipt := range []string{
		"preview copy rerun authoritative load",
		"failure rollback source preserved",
		"hash preview failure source preserved",
		"hash failure rollback source preserved",
		"constraint failure rollback source preserved",
		"commit failure rollback cursor preserved",
		"stale destination skipped",
		"bounded batch and cancellation",
		"mid batch cancellation rollback source preserved",
		"final save cancellation rollback source preserved",
		"multi page retry cursor copies exactly once",
		"wrong type returns typed source failure",
	} {
		if !strings.Contains(result.stdout, receipt) {
			t.Errorf("runtime output missing %q receipt\n%s", receipt, result.stdout)
		}
	}
}

func checkGeneratedMigrationWithGopls(t *testing.T, project generatedProject) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(project.root, "internal", "storage", "*.go"))
	if err != nil {
		t.Fatalf("list generated storage files for gopls: %v", err)
	}
	for _, file := range files {
		result := project.run(t, "generated-migration-gopls", project.root, "gopls", "check", file)
		if result.err != nil {
			t.Fatalf("%s", result.failureMessage())
		}
	}
}

const dedicatedMigrationTokenSource = `package v1

import (
	"context"
	"errors"

	"github.com/openchami/fabrica/pkg/fabrica"
)

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	APIVersion string           ` + "`json:\"apiVersion\"`" + `
	Kind       string           ` + "`json:\"kind\"`" + `
	Metadata   fabrica.Metadata ` + "`json:\"metadata\"`" + `
	Spec       TokenSpec        ` + "`json:\"spec\"`" + `
	Status     TokenStatus      ` + "`json:\"status,omitempty\"`" + `
}

type TokenSpec struct {
	// +fabrica:field:unique
	DisplayName string ` + "`json:\"displayName\" validate:\"required\"`" + `

	// +fabrica:field:storage=hashed:bcrypt:cost=4
	Password string ` + "`json:\"password\" validate:\"required\"`" + `

	// +fabrica:field:sensitive
	SensitiveNote string ` + "`json:\"sensitiveNote\"`" + `
}

type TokenStatus struct {
	State string ` + "`json:\"state\"`" + `
}

func (r *Token) Validate(ctx context.Context) error {
	if err := ctx.Err(); err != nil { return err }
	if r.Spec.DisplayName == "" || r.Spec.Password == "" { return errors.New("token fields are required") }
	return nil
}
func (r *Token) GetKind() string { return "Token" }
func (r *Token) GetName() string { return r.Metadata.Name }
func (r *Token) GetUID() string  { return r.Metadata.UID }
func (r *Token) IsHub()          {}
`
