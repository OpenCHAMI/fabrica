// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileBackend_PathTraversalPrevention tests that the file backend prevents path traversal attacks
func TestFileBackend_PathTraversalPrevention(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	testData := json.RawMessage(`{"name":"test","value":"data"}`)

	tests := []struct {
		name         string
		resourceType string
		uid          string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "path traversal with double dots in resourceType",
			resourceType: "../../../etc/passwd",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "path traversal with slash in resourceType",
			resourceType: "users/../../etc/passwd",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "path traversal with backslash in resourceType (Windows)",
			resourceType: "users\\..\\..\\etc\\passwd",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "path traversal in UID",
			resourceType: "users",
			uid:          "../../../etc/passwd",
			wantErr:      true,
			errContains:  "invalid UID",
		},
		{
			name:         "path traversal with slash in UID",
			resourceType: "users",
			uid:          "test/../../etc/passwd",
			wantErr:      true,
			errContains:  "invalid UID",
		},
		{
			name:         "resourceType with special characters",
			resourceType: "user$%^&*()",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "resourceType with dot prefix",
			resourceType: ".hidden",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "resourceType is just dots",
			resourceType: "..",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "resourceType is single dot",
			resourceType: ".",
			uid:          "test-123",
			wantErr:      true,
			errContains:  "invalid resource type",
		},
		{
			name:         "valid alphanumeric resourceType",
			resourceType: "User",
			uid:          "test-123",
			wantErr:      false,
		},
		{
			name:         "valid resourceType with hyphen",
			resourceType: "api-key",
			uid:          "test-123",
			wantErr:      false,
		},
		{
			name:         "valid resourceType with underscore",
			resourceType: "user_profile",
			uid:          "test-123",
			wantErr:      false,
		},
		{
			name:         "valid resourceType with numbers",
			resourceType: "v1Resource",
			uid:          "test-123",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Save operation
			err := backend.Save(ctx, tt.resourceType, tt.uid, testData)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Save() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Save() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Save() unexpected error = %v", err)
				}
			}

			// Test Load operation
			_, err = backend.Load(ctx, tt.resourceType, tt.uid)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) && err != ErrNotFound {
					t.Errorf("Load() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Load() unexpected error = %v", err)
				}
			}

			// Test Delete operation
			err = backend.Delete(ctx, tt.resourceType, tt.uid)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Delete() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) && err != ErrNotFound {
					t.Errorf("Delete() error = %v, want error containing %q", err, tt.errContains)
				}
			} else if err != nil && err != ErrNotFound {
				t.Errorf("Delete() unexpected error = %v", err)
			}

			// Test Exists operation
			_, err = backend.Exists(ctx, tt.resourceType, tt.uid)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Exists() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Exists() error = %v, want error containing %q", err, tt.errContains)
				}
			} else if err != nil {
				t.Errorf("Exists() unexpected error = %v", err)
			}

			// Test List operation
			// Note: List only validates resourceType, not UID
			_, err = backend.List(ctx, tt.resourceType)
			if tt.wantErr && strings.Contains(tt.errContains, "resource type") {
				if err == nil {
					t.Errorf("List() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("List() error = %v, want error containing %q", err, tt.errContains)
				}
			} else if !tt.wantErr && err != nil {
				t.Errorf("List() unexpected error = %v", err)
			}
			// List doesn't take UID parameter, so UID validation errors don't apply

			// Test LoadAll operation
			// Note: LoadAll only validates resourceType, not UID
			_, err = backend.LoadAll(ctx, tt.resourceType)
			if tt.wantErr && strings.Contains(tt.errContains, "resource type") {
				if err == nil {
					t.Errorf("LoadAll() expected error containing %q, got nil", tt.errContains)
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("LoadAll() error = %v, want error containing %q", err, tt.errContains)
				}
			} else if !tt.wantErr && err != nil {
				t.Errorf("LoadAll() unexpected error = %v", err)
			}
			// LoadAll doesn't take UID parameter, so UID validation errors don't apply
		})
	}
}

// TestFileBackend_PathTraversalDoesNotEscapeBaseDir verifies that even with valid characters,
// files cannot escape the base directory
func TestFileBackend_PathTraversalDoesNotEscapeBaseDir(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	testData := json.RawMessage(`{"name":"test"}`)

	// Try to save with a valid resource type
	resourceType := "User"
	uid := "test-123"

	err = backend.Save(ctx, resourceType, uid, testData)
	if err != nil {
		t.Fatalf("Failed to save test data: %v", err)
	}

	// Verify the file was created in the expected location (inside base directory)
	expectedDir := filepath.Join(tempDir, "users") // "User" -> "users"
	expectedFile := filepath.Join(expectedDir, "test-123.json")

	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedFile)
	}

	// Verify the file is actually within the base directory
	absExpected, err := filepath.Abs(expectedFile)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	absBase, err := filepath.Abs(tempDir)
	if err != nil {
		t.Fatalf("Failed to get absolute base path: %v", err)
	}

	if !strings.HasPrefix(absExpected, absBase) {
		t.Errorf("File path %s is not within base directory %s", absExpected, absBase)
	}

	// Verify no files were created outside the base directory
	parentDir := filepath.Dir(tempDir)
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		t.Fatalf("Failed to read parent directory: %v", err)
	}

	// Check that no unexpected files or directories were created
	for _, entry := range entries {
		entryPath := filepath.Join(parentDir, entry.Name())
		if entryPath != tempDir && strings.HasPrefix(entry.Name(), "test") {
			t.Errorf("Unexpected file/directory created outside base dir: %s", entryPath)
		}
	}
}

// TestFileBackend_BasicOperations tests basic CRUD operations work correctly
func TestFileBackend_BasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	resourceType := "User"
	uid := "user-123"
	testData := json.RawMessage(`{"name":"John","email":"john@example.com"}`)

	// Test Save
	err = backend.Save(ctx, resourceType, uid, testData)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Test Exists (should return true)
	exists, err := backend.Exists(ctx, resourceType, uid)
	if err != nil {
		t.Fatalf("Exists() failed: %v", err)
	}
	if !exists {
		t.Error("Exists() returned false, want true")
	}

	// Test Load
	loaded, err := backend.Load(ctx, resourceType, uid)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if string(loaded) != string(testData) {
		t.Errorf("Load() returned %s, want %s", loaded, testData)
	}

	// Test List
	uids, err := backend.List(ctx, resourceType)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(uids) != 1 || uids[0] != uid {
		t.Errorf("List() returned %v, want [%s]", uids, uid)
	}

	// Test LoadAll
	resources, err := backend.LoadAll(ctx, resourceType)
	if err != nil {
		t.Fatalf("LoadAll() failed: %v", err)
	}
	if len(resources) != 1 {
		t.Errorf("LoadAll() returned %d resources, want 1", len(resources))
	}
	if string(resources[0]) != string(testData) {
		t.Errorf("LoadAll() returned %s, want %s", resources[0], testData)
	}

	// Test Delete
	err = backend.Delete(ctx, resourceType, uid)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Test Exists (should return false after delete)
	exists, err = backend.Exists(ctx, resourceType, uid)
	if err != nil {
		t.Fatalf("Exists() after delete failed: %v", err)
	}
	if exists {
		t.Error("Exists() returned true after delete, want false")
	}

	// Test Load (should return ErrNotFound after delete)
	_, err = backend.Load(ctx, resourceType, uid)
	if err != ErrNotFound {
		t.Errorf("Load() after delete returned error %v, want ErrNotFound", err)
	}
}

// TestFileBackend_InvalidJSON tests that invalid JSON is rejected
func TestFileBackend_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	invalidData := json.RawMessage(`{invalid json}`)

	err = backend.Save(ctx, "User", "test-123", invalidData)
	if err == nil {
		t.Error("Save() with invalid JSON should fail")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("Save() error should mention invalid JSON, got: %v", err)
	}
}

// TestFileBackend_ConcurrentAccess tests thread-safety of the file backend
func TestFileBackend_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	resourceType := "User"
	testData := json.RawMessage(`{"name":"test"}`)

	// Spawn multiple goroutines performing operations concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			uid := string(rune('a' + id))
			_ = backend.Save(ctx, resourceType, uid, testData)
			_, _ = backend.Load(ctx, resourceType, uid)
			_, _ = backend.Exists(ctx, resourceType, uid)
			_, _ = backend.List(ctx, resourceType)
			_ = backend.Delete(ctx, resourceType, uid)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestFileBackend_ClosedBackend tests that operations fail on a closed backend
func TestFileBackend_ClosedBackend(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}

	// Close the backend
	err = backend.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	ctx := context.Background()
	testData := json.RawMessage(`{"name":"test"}`)

	// All operations should fail on closed backend
	err = backend.Save(ctx, "User", "test", testData)
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Save() on closed backend should fail with 'closed' error, got: %v", err)
	}

	_, err = backend.Load(ctx, "User", "test")
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Load() on closed backend should fail with 'closed' error, got: %v", err)
	}

	_, err = backend.Exists(ctx, "User", "test")
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Exists() on closed backend should fail with 'closed' error, got: %v", err)
	}

	_, err = backend.List(ctx, "User")
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("List() on closed backend should fail with 'closed' error, got: %v", err)
	}

	_, err = backend.LoadAll(ctx, "User")
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("LoadAll() on closed backend should fail with 'closed' error, got: %v", err)
	}

	err = backend.Delete(ctx, "User", "test")
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Delete() on closed backend should fail with 'closed' error, got: %v", err)
	}
}

// TestFileBackend_ContextCancellation tests that operations respect context cancellation
func TestFileBackend_ContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	testData := json.RawMessage(`{"name":"test"}`)

	// Operations should respect cancellation
	err = backend.Save(ctx, "User", "test", testData)
	if err == nil || err != context.Canceled {
		t.Errorf("Save() should return context.Canceled, got: %v", err)
	}

	_, err = backend.Load(ctx, "User", "test")
	if err == nil || err != context.Canceled {
		t.Errorf("Load() should return context.Canceled, got: %v", err)
	}

	_, err = backend.Exists(ctx, "User", "test")
	if err == nil || err != context.Canceled {
		t.Errorf("Exists() should return context.Canceled, got: %v", err)
	}

	_, err = backend.List(ctx, "User")
	if err == nil || err != context.Canceled {
		t.Errorf("List() should return context.Canceled, got: %v", err)
	}

	_, err = backend.LoadAll(ctx, "User")
	if err == nil || err != context.Canceled {
		t.Errorf("LoadAll() should return context.Canceled, got: %v", err)
	}

	err = backend.Delete(ctx, "User", "test")
	if err == nil || err != context.Canceled {
		t.Errorf("Delete() should return context.Canceled, got: %v", err)
	}
}

// TestFileBackend_EmptyResourceType tests handling of empty resource type
func TestFileBackend_EmptyResourceType(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewFileBackend(tempDir)
	if err != nil {
		t.Fatalf("Failed to create file backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	testData := json.RawMessage(`{"name":"test"}`)

	err = backend.Save(ctx, "", "test", testData)
	if err == nil {
		t.Error("Save() with empty resourceType should fail")
	}
}
