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
	run(project, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo)
	run(project, binary, "add", "resource", "Token")
	run(project, binary, "add", "resource", "Widget")
	run(project, binary, "add", "resource", "Node")
	run(project, binary, "generate", "--fabrica-source", repo)
	migration, err := os.ReadFile(filepath.Join(repo, "cmd/fabrica/testdata/storage_migration_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(project, "cmd/server/storage_migration_test.go"), migration, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FABRICA_TEST_MIGRATION_DIR", filepath.Join(workspace, "migration"))
	legacyTests := filepath.Join(workspace, "legacy-storage.test")
	run(project, "go", "test", "-mod=mod", "-c", "-o", legacyTests, "./cmd/server")
	t.Setenv("FABRICA_TEST_MIGRATION_STAGE", "export")
	run(project, legacyTests, "-test.run", "^TestStorageMigration$")
	types := filepath.Join(project, "apis", "example.fabrica.dev", "v1", "token_types.go")
	source, err := os.ReadFile(types)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(source), "// Token represents", "// +fabrica:resource\n// +fabrica:storage=dedicated\n// Token represents", 1)
	text = strings.Replace(text, "Description string", "// +fabrica:field:unique\n\tDescription string", 1)
	text = strings.Replace(text, "// Add your spec fields here", "// +fabrica:field:storage=hashed:bcrypt:cost=4\nPassword string `json:\"password,omitempty\"`\n// +fabrica:field:immutable\nOwner string `json:\"owner,omitempty\"`\n// +fabrica:field:default=user\nRole string `json:\"role,omitempty\"`", 1)
	if err = os.WriteFile(types, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	nodeTypes := filepath.Join(project, "apis", "example.fabrica.dev", "v1", "node_types.go")
	nodeSource, err := os.ReadFile(nodeTypes)
	if err != nil {
		t.Fatal(err)
	}
	nodeSource = []byte(strings.Replace(string(nodeSource), "// Node represents", "// +fabrica:resource\n// +fabrica:storage=dedicated\n// Node represents", 1))
	nodeSource = []byte(strings.Replace(string(nodeSource), "Description string", "// +fabrica:field:index\nDescription string", 1))
	if err = os.WriteFile(nodeTypes, nodeSource, 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		run(project, binary, "generate", "--fabrica-source", repo)
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
	routing, err := os.ReadFile(filepath.Join(repo, "cmd/fabrica/testdata/dedicated_routing_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(project, "internal/storage/dedicated_routing_test.go"), routing, 0644); err != nil {
		t.Fatal(err)
	}
	run(project, "go", "test", "-mod=mod", "-race", "./internal/storage")
	transfer, err := os.ReadFile(filepath.Join(repo, "cmd/fabrica/testdata/dedicated_transfer_guard_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(project, "cmd/server/dedicated_transfer_guard_test.go"), transfer, 0644); err != nil {
		t.Fatal(err)
	}
	run(project, "go", "test", "-mod=mod", "./cmd/server", "-run", "^TestDedicatedTransferGuard$")
	t.Setenv("FABRICA_TEST_MIGRATION_STAGE", "import")
	run(project, "go", "test", "-mod=mod", "./cmd/server", "-run", "^TestStorageMigration$")
	t.Setenv("FABRICA_TEST_MIGRATION_STAGE", "rollback")
	run(project, legacyTests, "-test.run", "^TestStorageMigration$")

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
