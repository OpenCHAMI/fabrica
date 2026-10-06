// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
)

func TestGeneratedEntAtomicMutations(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(original, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Error(err)
		}
	})
	module := "module example.com/atomic-ent\n\ngo 1.26.6\n\nrequire (\n github.com/openchami/fabrica v0.0.0\n entgo.io/ent v0.14.5\n github.com/mattn/go-sqlite3 v1.14.32\n)\nreplace github.com/openchami/fabrica => " + root + "\n"
	if err := os.WriteFile("go.mod", []byte(module), 0644); err != nil {
		t.Fatal(err)
	}
	gen := NewGenerator(dir, "test", "example.com/atomic-ent")
	gen.StorageType = "ent"
	gen.DBDriver = "sqlite3"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.RegisterResource(&testfixtures.MappedToken{}); err != nil {
		t.Fatal(err)
	}
	for _, generate := range []func() error{gen.GenerateEntSchemas, gen.GenerateEntAdapter, gen.GenerateStorage} {
		if err := generate(); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(filepath.Join(original, "testdata", "atomic_ent_runtime_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "storage", "atomic_test.go"), fixture, 0644); err != nil {
		t.Fatal(err)
	}
	runInDir(t, dir, "go", "run", "-mod=mod", "entgo.io/ent/cmd/ent", "generate", "./internal/storage/ent/schema")
	runInDir(t, dir, "go", "mod", "tidy")
	runInDir(t, dir, "go", "test", "-race", "-count=1", "./internal/storage")
	// Reuse the generated schema to exercise compiled storage with custom limits.
	for _, limit := range []int{1, 3} {
		gen.Config.MutationMaxAttempts = limit
		if err := gen.GenerateStorage(); err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join(dir, "internal", "storage", "storage_generated.go"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), fmt.Sprintf("const entMutationMaxAttempts = %d", limit)) {
			t.Fatalf("configured attempt limit %d did not reach generated storage", limit)
		}
		runInDir(t, dir, "go", "test", "-race", "-count=1", "-run", "^TestEntMutationRetryLimit$", "./internal/storage")
	}
}
