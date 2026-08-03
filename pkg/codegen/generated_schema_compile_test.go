// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/annotations"
)

// entVersion is the entgo.io/ent release the generated schemas are verified
// against. Bump deliberately: a generated schema that compiles against one Ent
// release is not guaranteed to compile against the next.
const entVersion = "v0.14.6"

// These tests answer a question the golden tests cannot: does the code we
// generate actually build?
//
// Golden tests compare text. executeTemplate runs format.Source, which checks
// syntax only — unused imports, missing imports and undefined identifiers all
// pass gofmt. Every dedicated schema fabrica emitted before this file existed
// was unbuildable, and nothing caught it, because no test ever handed the output
// to a compiler.
//
// They shell out to `go build` in a temp module, so they need a Go toolchain and
// entgo.io/ent resolvable (from the module cache or the network). Skipped in
// -short mode.

// newSchemaModule writes a throwaway module containing a schema package and
// returns its directory. Files are given as name → content.
func newSchemaModule(t *testing.T, files map[string][]byte) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping compile test in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain available")
	}

	dir := t.TempDir()
	schemaDir := filepath.Join(dir, "schema")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(schemaDir, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	gomod := "module fabricaschematest\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Resolve dependencies. If this fails the environment cannot reach Ent, so
	// skip rather than report a false failure.
	for _, args := range [][]string{
		{"get", "entgo.io/ent@" + entVersion},
		{"get", "golang.org/x/crypto"},
		{"mod", "tidy"},
	} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("cannot resolve Ent dependencies (%v): %s", err, out)
		}
	}

	return dir
}

// buildSchemaModule runs `go build` over the schema package and returns the
// combined output plus whether it succeeded.
func buildSchemaModule(t *testing.T, dir string) (string, bool) {
	t.Helper()

	cmd := exec.Command("go", "build", "./schema/")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	return string(out), err == nil
}

// TestGeneratedGoldenSchemasCompile builds every golden schema against a real
// Ent release. This is the regression guard for the class of bug the goldens
// missed entirely.
func TestGeneratedGoldenSchemasCompile(t *testing.T) {
	goldenDir := filepath.Join("testdata", "golden")

	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}

	files := make(map[string][]byte)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go.golden") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(goldenDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		// Each golden declares a distinct type, so they coexist in one package.
		files[strings.TrimSuffix(e.Name(), ".golden")] = content
	}

	if len(files) == 0 {
		t.Fatal("no golden schemas found to compile")
	}

	dir := newSchemaModule(t, files)
	if out, ok := buildSchemaModule(t, dir); !ok {
		t.Errorf("generated golden schemas do not compile against entgo.io/ent %s:\n%s", entVersion, out)
	}
}

// noStorageSpec has no hashed or encrypted field, so the generated schema must
// NOT import dialect or bcrypt. This is the configuration that used to fail with
// "imported and not used" — the common case, and the one both tokensmith
// resources hit.
type noStorageSpec struct {
	Name      string    `json:"name" validate:"required"`
	CreatedBy string    `json:"created_by"`
	ExpiresAt time.Time `json:"expires_at"`
}

type noStorageResource struct {
	Spec noStorageSpec
}

// TestSchemaWithoutHashedFieldsCompiles pins the unused-import fix.
func TestSchemaWithoutHashedFieldsCompiles(t *testing.T) {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	name := annotations.NewFieldAnnotations("Name")
	name.Size = 253
	annots.Fields["Name"] = name

	got := generateDedicatedSchema(t, &noStorageResource{}, "noStorageResource", annots)

	for _, unwanted := range []string{`"entgo.io/ent/dialect"`, `"golang.org/x/crypto/bcrypt"`, `"context"`} {
		if strings.Contains(string(got), unwanted) {
			t.Errorf("schema with no hashed fields should not import %s:\n%s", unwanted, got)
		}
	}

	dir := newSchemaModule(t, map[string][]byte{"nostorage.go": got})
	if out, ok := buildSchemaModule(t, dir); !ok {
		t.Errorf("schema without hashed fields does not compile:\n%s", out)
	}
}

type sha256Spec struct {
	Token string `json:"token" validate:"required"`
}

type sha256Resource struct {
	Spec sha256Spec
}

// TestSchemaWithSHA256HashCompiles covers the other import branch: a hashed
// field needs context, sha256 and hex, none of which were imported before.
func TestSchemaWithSHA256HashCompiles(t *testing.T) {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	token := annotations.NewFieldAnnotations("Token")
	token.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmSHA256},
	}
	token.Sensitive = true
	annots.Fields["Token"] = token

	got := generateDedicatedSchema(t, &sha256Resource{}, "sha256Resource", annots)

	for _, want := range []string{`"crypto/sha256"`, `"encoding/hex"`, `"context"`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("sha256 schema is missing import %s:\n%s", want, got)
		}
	}
	// bcrypt is not used by the sha256 path and must not be imported.
	if strings.Contains(string(got), `"golang.org/x/crypto/bcrypt"`) {
		t.Error("sha256 schema should not import bcrypt")
	}

	dir := newSchemaModule(t, map[string][]byte{"sha256.go": got})
	if out, ok := buildSchemaModule(t, dir); !ok {
		t.Errorf("sha256 schema does not compile:\n%s", out)
	}
}

// TestHooksDoNotReferenceGeneratedPackage pins the layering fix. The hook used
// to assert on *ent.<Name>Mutation, a type generated FROM the schema package, so
// the schema could never compile. It now matches the mutation structurally.
func TestHooksDoNotReferenceGeneratedPackage(t *testing.T) {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	token := annotations.NewFieldAnnotations("Token")
	token.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmBcrypt, Cost: 12},
	}
	annots.Fields["Token"] = token

	got := string(generateDedicatedSchema(t, &sha256Resource{}, "sha256Resource", annots))

	// Look for a real type assertion, not the explanatory comment that names
	// the type it deliberately avoids.
	if strings.Contains(got, "m.(*ent.") {
		t.Error("hook asserts on the generated mutation type; the schema cannot compile against it")
	}
	if !strings.Contains(got, "m.(interface {") {
		t.Errorf("hook should match the mutation structurally:\n%s", got)
	}
}
