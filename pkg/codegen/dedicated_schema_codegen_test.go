//go:build integration

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const dedicatedSchemaCommandTimeout = 2 * time.Minute

func TestGeneratedDedicatedSchema_passes_ent_codegen_and_build_by_dialect(t *testing.T) {
	tests := []struct {
		name   string
		driver string
	}{
		{name: "PostgreSQL", driver: "postgres"},
		{name: "SQLite", driver: "sqlite"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			fixture := generateDedicatedShapeSchema(t, test.driver)
			goMod := `module example.com/dedicated-schema-fixture

go 1.24.0

require entgo.io/ent v0.14.5
`
			if err := os.WriteFile(filepath.Join(fixture.root, "go.mod"), []byte(goMod), 0o644); err != nil {
				t.Fatalf("write schema fixture go.mod: %v", err)
			}

			// When
			runDedicatedSchemaCommand(t, fixture.root, "go", "mod", "tidy")
			runDedicatedSchemaCommand(
				t,
				fixture.root,
				"go",
				"run",
				"entgo.io/ent/cmd/ent@v0.14.5",
				"generate",
				"./internal/storage/ent/schema",
			)
			runDedicatedSchemaCommand(t, fixture.root, "go", "mod", "tidy")
			runDedicatedSchemaCommand(t, fixture.root, "go", "build", "./internal/storage/ent/...")

			// Then
			for _, artifact := range []string{
				filepath.Join("internal", "storage", "ent", "dedicatedshape.go"),
				filepath.Join("internal", "storage", "ent", "dedicatedshape", "where.go"),
			} {
				if _, err := os.Stat(filepath.Join(fixture.root, artifact)); err != nil {
					t.Errorf("Ent codegen artifact %s: %v", artifact, err)
				}
			}
		})
	}
}

func runDedicatedSchemaCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), dedicatedSchemaCommandTimeout)
	defer cancel()

	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %v\n%s", fmt.Sprintf("%s %v", name, args), err, output.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("%s %v timed out after %s", name, args, dedicatedSchemaCommandTimeout)
	}
}
