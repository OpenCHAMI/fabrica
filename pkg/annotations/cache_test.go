// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnnotationCache(t *testing.T) {
	cache := NewAnnotationCache()

	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte("package test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test cache miss
	if _, ok := cache.Get(tmpFile); ok {
		t.Error("Expected cache miss")
	}

	// Set cache
	anns := map[string]*ResourceAnnotations{
		"TestResource": NewResourceAnnotations(),
	}
	cache.Set(tmpFile, anns)

	// Test cache hit
	if got, ok := cache.Get(tmpFile); !ok {
		t.Error("Expected cache hit")
	} else if len(got) != 1 {
		t.Errorf("Got %d annotations, want 1", len(got))
	}

	// Modify file (update mtime)
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(tmpFile, []byte("package test\n// modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test cache invalidation (automatic on Get)
	if _, ok := cache.Get(tmpFile); ok {
		t.Error("Expected cache miss after file modification")
	}
}

func TestAnnotationCache_Clear(t *testing.T) {
	cache := NewAnnotationCache()

	cache.Set("file1.go", map[string]*ResourceAnnotations{})
	cache.Set("file2.go", map[string]*ResourceAnnotations{})

	if cache.Size() != 2 {
		t.Errorf("Size() = %d, want 2", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Size() after Clear() = %d, want 0", cache.Size())
	}
}

func TestAnnotationCache_Invalidate(t *testing.T) {
	cache := NewAnnotationCache()

	// Create temp files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")

	if err := os.WriteFile(file1, []byte("package test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("package test"), 0644); err != nil {
		t.Fatal(err)
	}

	cache.Set(file1, map[string]*ResourceAnnotations{})
	cache.Set(file2, map[string]*ResourceAnnotations{})

	cache.Invalidate(file1)

	if cache.Size() != 1 {
		t.Errorf("Size() after Invalidate() = %d, want 1", cache.Size())
	}

	if _, ok := cache.Get(file1); ok {
		t.Error("Expected cache miss for invalidated file")
	}

	if _, ok := cache.Get(file2); !ok {
		t.Error("Expected cache hit for non-invalidated file")
	}
}

func TestParseFileAnnotationsCachePreservesVocabularyFields(t *testing.T) {
	globalCache.Clear()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "types.go")
	source := `package test

type OwnerID string

// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:migration=additive-only
// +fabrica:index:fields=OwnerID,CreatedAt:name=idx_owner_created:unique
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// +fabrica:field:nullable
	// +fabrica:field:size=128
	// +fabrica:field:relation=belongs-to:User:on-delete=cascade
	OwnerID OwnerID
	CreatedAt string
}
`
	if err := os.WriteFile(tmpFile, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := ParseFileAnnotations(tmpFile)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	second, err := ParseFileAnnotations(tmpFile)
	if err != nil {
		t.Fatalf("cached parse: %v", err)
	}

	for label, anns := range map[string]*ResourceAnnotations{"first": first["Token"], "cached": second["Token"]} {
		t.Run(label, func(t *testing.T) {
			if anns == nil {
				t.Fatal("Token annotations missing")
			}
			if anns.Migration != MigrationPolicyAdditiveOnly {
				t.Fatalf("Migration = %q, want additive-only", anns.Migration)
			}
			if len(anns.Indexes) != 1 {
				t.Fatalf("Indexes length = %d, want 1", len(anns.Indexes))
			}
			idx := anns.Indexes[0]
			if idx.Name != "idx_owner_created" || !idx.Unique {
				t.Fatalf("Index = %+v, want named unique", idx)
			}
			if !anns.SpecFields["OwnerID"] || !anns.SpecFields["CreatedAt"] {
				t.Fatalf("SpecFields = %+v, want OwnerID and CreatedAt", anns.SpecFields)
			}
			owner := anns.Fields["OwnerID"]
			if owner == nil || !owner.Nullable || owner.Size != 128 {
				t.Fatalf("OwnerID annotations = %+v", owner)
			}
			if !owner.TypeInfo.IsStringLike || owner.TypeInfo.UnderlyingKind != FieldKindString || owner.TypeInfo.NamedType != "OwnerID" {
				t.Fatalf("OwnerID type info = %+v", owner.TypeInfo)
			}
			if owner.Relation == nil || owner.Relation.OnDelete != OnDeleteCascade {
				t.Fatalf("OwnerID relation = %+v", owner.Relation)
			}
		})
	}
}

func TestParseFileAnnotationsCacheInvalidatesOnPackageDependencyChange(t *testing.T) {
	globalCache.Clear()

	dir := t.TempDir()
	aliasPath := filepath.Join(dir, "aliases.go")
	typesPath := filepath.Join(dir, "types.go")
	if err := os.WriteFile(aliasPath, []byte("package test\n\ntype Email string\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(typesPath, []byte(`package test

// +fabrica:resource
// +fabrica:storage=dedicated
type User struct { Spec UserSpec }

type UserSpec struct {
	// +fabrica:field:size=253
	Email Email
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := ParseFileAnnotations(typesPath)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if err := Validate(first["User"]); err != nil {
		t.Fatalf("first validation: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(aliasPath, []byte("package test\n\ntype Email int\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := ParseFileAnnotations(typesPath)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	err = Validate(second["User"])
	if err == nil || !strings.Contains(err.Error(), "size requires a string field") {
		t.Fatalf("expected cache miss and non-string validation error, got %v", err)
	}
}

func TestAnnotationCache_InvalidatesOnAddedPackageFile(t *testing.T) {
	// Use globalCache since that's what ParseFileAnnotations uses
	GetGlobalCache().Clear()

	dir := t.TempDir()
	typesPath := filepath.Join(dir, "types.go")
	aliasPath := filepath.Join(dir, "aliases.go")

	// Write only types.go initially
	if err := os.WriteFile(typesPath, []byte(`package test

// +fabrica:resource
type User struct { Spec UserSpec }

type UserSpec struct {
	Value string
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Parse and cache
	_, err := ParseFileAnnotations(typesPath)
	if err != nil {
		t.Fatalf("initial parse: %v", err)
	}

	// Verify cache hit
	if _, ok := GetGlobalCache().Get(typesPath); !ok {
		t.Fatal("expected cache hit for initial state")
	}

	// Add aliases.go
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(aliasPath, []byte("package test\n\ntype Email string\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Cache should miss because package file set changed
	if _, ok := GetGlobalCache().Get(typesPath); ok {
		t.Fatal("expected cache miss after adding package file")
	}
}

func TestAnnotationCache_InvalidatesOnRemovedPackageFile(t *testing.T) {
	globalCache.Clear()

	dir := t.TempDir()
	typesPath := filepath.Join(dir, "types.go")
	aliasPath := filepath.Join(dir, "aliases.go")

	// Write both files initially
	if err := os.WriteFile(aliasPath, []byte("package test\n\ntype Email string\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(typesPath, []byte(`package test

// +fabrica:resource
// +fabrica:storage=dedicated
type User struct { Spec UserSpec }

type UserSpec struct {
	// +fabrica:field:size=253
	Value string
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Parse and cache with both files
	_, err := ParseFileAnnotations(typesPath)
	if err != nil {
		t.Fatalf("initial parse: %v", err)
	}

	// Verify cache hit
	if _, ok := globalCache.Get(typesPath); !ok {
		t.Fatal("expected cache hit for initial state")
	}

	// Remove aliases.go
	time.Sleep(10 * time.Millisecond)
	if err := os.Remove(aliasPath); err != nil {
		t.Fatal(err)
	}

	// Cache should miss because package file set changed
	if _, ok := globalCache.Get(typesPath); ok {
		t.Fatal("expected cache miss after removing package file")
	}
}
