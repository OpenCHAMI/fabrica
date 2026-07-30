// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestCache_baseline_preserves_existing_field_cache_API(t *testing.T) {
	// Given
	cache := NewAnnotationCache()
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte("package test"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := map[string]*FieldAnnotations{
		"WidgetSpec.Name": {FieldName: "Name", Sensitive: true},
	}

	// When
	cache.Set(filename, want)
	got, ok := cache.Get(filename)

	// Then
	if !ok {
		t.Fatal("Get() cache hit = false, want true")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}
}

func TestParseFileAnnotations_baseline_cold_parse_returns_resource_and_field_annotations(t *testing.T) {
	// Given
	globalCache.Clear()
	t.Cleanup(globalCache.Clear)
	filename := filepath.Join(t.TempDir(), "types.go")
	source := `package test

// +fabrica:resource
// +fabrica:storage=dedicated
type Widget struct {
	Spec WidgetSpec
}

type WidgetSpec struct {
	// +fabrica:field:sensitive
	Name string ` + "`json:\"name\"`" + `
}
`
	if err := os.WriteFile(filename, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	// When
	got, err := ParseFileAnnotations(filename)

	// Then
	if err != nil {
		t.Fatalf("ParseFileAnnotations() error = %v", err)
	}
	if !got["Widget"].IsResource {
		t.Error("Widget.IsResource = false, want true")
	}
	if got["Widget"].StorageMode != StorageModeDedicated {
		t.Errorf("Widget.StorageMode = %q, want %q", got["Widget"].StorageMode, StorageModeDedicated)
	}
	if !got["WidgetSpec"].Fields["Name"].Sensitive {
		t.Error("WidgetSpec.Name.Sensitive = false, want true")
	}
}

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
	anns := map[string]*FieldAnnotations{
		"TestField": NewFieldAnnotations("TestField"),
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

	cache.Set("file1.go", map[string]*FieldAnnotations{})
	cache.Set("file2.go", map[string]*FieldAnnotations{})

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

	cache.Set(file1, map[string]*FieldAnnotations{})
	cache.Set(file2, map[string]*FieldAnnotations{})

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

func TestCache_concurrent_gets_return_isolated_field_annotations(t *testing.T) {
	// Given
	cache := NewAnnotationCache()
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte("package test"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache.Set(filename, map[string]*FieldAnnotations{
		"WidgetSpec.Name": {FieldName: "Name"},
	})
	const readers = 32
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(readers)

	// When
	for i := 0; i < readers; i++ {
		go func(value bool) {
			defer workers.Done()
			<-start
			got, ok := cache.Get(filename)
			if !ok {
				return
			}
			got["WidgetSpec.Name"].Sensitive = value
		}(i%2 == 0)
	}
	close(start)
	workers.Wait()

	// Then
	got, ok := cache.Get(filename)
	if !ok {
		t.Fatal("Get() cache hit = false, want true")
	}
	if got["WidgetSpec.Name"].Sensitive {
		t.Fatal("concurrent caller mutation changed cached annotation")
	}
}

func TestCache_zero_value_supports_set_get_and_clear(t *testing.T) {
	// Given
	var cache AnnotationCache
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte("package test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When
	cache.Set(filename, map[string]*FieldAnnotations{"Name": {FieldName: "Name"}})
	got, ok := cache.Get(filename)

	// Then
	if !ok || got["Name"].FieldName != "Name" {
		t.Fatalf("zero-value cache Get() = %#v, %v", got, ok)
	}
	cache.Clear()
	if cache.Size() != 0 {
		t.Fatalf("zero-value cache Size() after Clear() = %d, want 0", cache.Size())
	}
}

func TestCache_set_and_get_defensively_copy_nested_field_annotations(t *testing.T) {
	// Given
	cache := NewAnnotationCache()
	filename := filepath.Join(t.TempDir(), "types.go")
	if err := os.WriteFile(filename, []byte("package test"), 0o644); err != nil {
		t.Fatal(err)
	}
	input := map[string]*FieldAnnotations{
		"Secret": {
			FieldName: "Secret",
			Storage: &StorageConfig{
				Type:       StorageTypeHashed,
				Hash:       &HashConfig{Algorithm: HashAlgorithmBcrypt, Cost: 12},
				Encryption: &EncryptionConfig{Algorithm: "aes256", KeySource: "env"},
			},
			Index:          &IndexConfig{Type: IndexTypeBTree, Name: "secret_idx"},
			RawAnnotations: []string{"+fabrica:field:sensitive"},
		},
	}

	// When
	cache.Set(filename, input)
	input["Secret"].Storage.Hash.Cost = 4
	input["Secret"].Storage.Encryption.KeySource = "corrupted"
	input["Secret"].Index.Name = "corrupted"
	input["Secret"].RawAnnotations[0] = "corrupted"
	first, ok := cache.Get(filename)
	if !ok {
		t.Fatal("first Get() cache hit = false, want true")
	}
	first["Secret"].Storage.Hash.Cost = 5
	first["Secret"].Storage.Encryption.KeySource = "mutated"
	first["Secret"].Index.Name = "mutated"
	first["Secret"].RawAnnotations[0] = "mutated"
	second, ok := cache.Get(filename)

	// Then
	if !ok {
		t.Fatal("second Get() cache hit = false, want true")
	}
	secret := second["Secret"]
	if secret.Storage.Hash.Cost != 12 || secret.Storage.Encryption.KeySource != "env" {
		t.Fatalf("nested storage pointers leaked mutation: %#v", secret.Storage)
	}
	if secret.Index.Name != "secret_idx" || secret.RawAnnotations[0] != "+fabrica:field:sensitive" {
		t.Fatalf("nested field data leaked mutation: %#v", secret)
	}
}
