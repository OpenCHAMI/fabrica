// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
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
