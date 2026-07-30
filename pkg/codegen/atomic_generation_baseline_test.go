// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicBaseline_valid_generation_is_byte_stable(t *testing.T) {
	// Given
	gen, root, _ := newStaleSchemaGenerator(t)
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("first GenerateEntSchemas() error = %v", err)
	}
	path := filepath.Join(root, "internal", "storage", "ent", "schema", "stalededicated.go")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first generated schema: %v", err)
	}

	// When
	err = gen.GenerateEntSchemas()

	// Then
	if err != nil {
		t.Fatalf("second GenerateEntSchemas() error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read second generated schema: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("valid repeated generation changed output bytes")
	}
}
