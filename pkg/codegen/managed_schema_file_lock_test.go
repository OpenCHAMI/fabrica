// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeManagedSchemaFileLock struct {
	locked    bool
	lockErr   error
	unlockErr error
}

func (f *fakeManagedSchemaFileLock) TryLock() (bool, error) {
	return f.locked, f.lockErr
}

func (f *fakeManagedSchemaFileLock) Unlock() error {
	return f.unlockErr
}

func TestManagedSchemaFileLock_busy_is_typed_and_does_not_mutate_output(t *testing.T) {
	// Given
	current := filepath.Join(t.TempDir(), "schema")
	writeManagedSchemaFixture(t, filepath.Join(current, "resource.go"), "type Before struct{}")
	ops := newManagedSchemaOperations(nil)
	ops.newFileLock = func(string) managedSchemaFileLock {
		return &fakeManagedSchemaFileLock{locked: false}
	}
	output := newManagedSchemaOutput(current, ops)

	// When
	_, err := output.acquireTransactionLock()

	// Then
	if !errors.Is(err, ErrManagedSchemaBusy) {
		t.Fatalf("acquireTransactionLock() error = %v, want busy", err)
	}
}

func TestManagedSchemaFileLock_unlock_error_is_returned_and_file_persists(t *testing.T) {
	// Given
	current := filepath.Join(t.TempDir(), "schema")
	unlockErr := errors.New("unlock fault")
	ops := newManagedSchemaOperations(nil)
	ops.newFileLock = func(path string) managedSchemaFileLock {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		return &fakeManagedSchemaFileLock{locked: true, unlockErr: unlockErr}
	}
	output := newManagedSchemaOutput(current, ops)
	release, err := output.acquireTransactionLock()
	if err != nil {
		t.Fatal(err)
	}

	// When
	err = release()

	// Then
	if !errors.Is(err, unlockErr) {
		t.Fatalf("release() error = %v, want unlock fault", err)
	}
	info, statErr := os.Lstat(output.lock)
	if statErr != nil || !info.Mode().IsRegular() {
		t.Fatalf("persistent lock path = %#v, %v", info, statErr)
	}
}

func TestManagedSchemaFileLock_rejects_symlink_or_nonregular_path(t *testing.T) {
	tests := []struct {
		name string
		make func(*testing.T, string)
	}{
		{name: "symlink", make: func(t *testing.T, path string) {
			target := filepath.Join(t.TempDir(), "target.lock")
			if err := os.WriteFile(target, []byte("sentinel"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "directory", make: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			current := filepath.Join(t.TempDir(), "schema")
			output := newManagedSchemaOutput(current, newManagedSchemaOperations(nil))
			if err := os.MkdirAll(filepath.Dir(output.lock), 0o755); err != nil {
				t.Fatal(err)
			}
			tt.make(t, output.lock)

			// When
			_, err := output.acquireTransactionLock()

			// Then
			if !errors.Is(err, errUnmanagedSchemaPath) {
				t.Fatalf("acquireTransactionLock() error = %v, want protected path", err)
			}
			if _, err := os.Lstat(output.lock); err != nil {
				t.Fatalf("protected lock path changed: %v", err)
			}
		})
	}
}
