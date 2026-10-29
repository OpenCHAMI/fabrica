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

func TestInvalidValidationPolicyPreservesOutputs(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		dir := t.TempDir()
		path := filepath.Join(dir, "models_generated.go")
		original := []byte("package main\n// existing output\n")
		if err := os.WriteFile(path, original, 0644); err != nil {
			t.Fatal(err)
		}
		gen := NewGenerator(dir, "main", "example.com/test")
		gen.Config.ValidationEnabled = enabled
		gen.Config.ValidationMode = "invalid"
		if err := gen.GenerateAll(); err == nil {
			t.Fatal("invalid policy accepted")
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatalf("rejection changed output: %s, %v", got, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("rejection added output: %v, %v", entries, err)
		}
	}
}
