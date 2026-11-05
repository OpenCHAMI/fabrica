// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the built CLI: parser-only tests cannot establish that generation
// actually loads annotations before emitting schemas and adapters.
func TestStorageAnnotationsCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("generated service integration")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	binary := filepath.Join(workspace, "fabrica")
	t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -mod=mod"))
	run := func(dir, executable string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", executable, args, err, out)
		}
		return out
	}
	run(repo, "go", "build", "-o", binary, "./cmd/fabrica")
	run(workspace, binary, "init", "annotated", "--module", "example.com/annotated", "--storage-type", "ent", "--db", "sqlite")
	project := filepath.Join(workspace, "annotated")
	run(project, binary, "add", "resource", "Token")
	types := filepath.Join(project, "apis", "example.fabrica.dev", "v1", "token_types.go")
	source, err := os.ReadFile(types)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(source), "// Token represents", "// +fabrica:resource\n// +fabrica:storage=dedicated\n// Token represents", 1)
	text = strings.Replace(text, "Description string", "// +fabrica:field:unique\n\tDescription string", 1)
	if err = os.WriteFile(types, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		run(project, binary, "generate", "--storage", "--fabrica-source", repo)
		schema, err := os.ReadFile(filepath.Join(project, "internal", "storage", "ent", "schema", "token.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(schema), `field.String("description")`) || !strings.Contains(string(schema), "Unique()") {
			t.Fatalf("dedicated constraint not emitted:\n%s", schema)
		}
		if _, err = os.Stat(filepath.Join(project, "internal", "storage", "ent_adapter_token.go")); err != nil {
			t.Fatal(err)
		}
	}
	run(project, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo)
	run(project, "go", "mod", "edit", "-require", "github.com/mattn/go-sqlite3@v1.14.32")
	fixture, err := os.ReadFile(filepath.Join(repo, "cmd/fabrica/testdata/dedicated_envelope_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(project, "internal/storage/dedicated_envelope_test.go"), fixture, 0644); err != nil {
		t.Fatal(err)
	}
	run(project, "go", "test", "-mod=mod", "./internal/storage")
	text = strings.Replace(text, "storage=dedicated", "storage=typo", 1)
	if err = os.WriteFile(types, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "generate", "--storage", "--fabrica-source", repo)
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unknown storage mode") {
		t.Fatalf("invalid annotation was not reported: %v\n%s", err, out)
	}
}
