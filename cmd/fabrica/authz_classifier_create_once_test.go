// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen"
)

func TestGenerateAuthZClassifier_CreateOncePreserved(t *testing.T) {
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "cmd", "server")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	userPath := filepath.Join(outDir, "authz_classifier.go")
	userContents := "// user customized\npackage main\n\n// custom\n"
	if err := os.WriteFile(userPath, []byte(userContents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gen := codegen.NewGenerator(outDir, "main", "example.com/test")
	gen.Version = "test"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}

	// Simulate generation attempting to create the classifier file; because it exists,
	// it must not be overwritten.
	if err := writeCreateOnceAuthZClassifier(gen, outDir); err != nil {
		t.Fatalf("writeCreateOnceAuthZClassifier: %v", err)
	}

	got, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != userContents {
		t.Fatalf("create-once file was overwritten; got=%q want=%q", string(got), userContents)
	}
}

func writeCreateOnceAuthZClassifier(gen *codegen.Generator, outDir string) error {
	// This mirrors the pattern used by reconciliation stubs (create if missing).
	path := filepath.Join(outDir, "authz_classifier.go")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmpl, ok := gen.Templates["authzClassifierCreateOnce"]
	if !ok {
		return os.ErrNotExist
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, map[string]any{}); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}
