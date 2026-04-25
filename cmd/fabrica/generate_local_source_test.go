// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCommandExposesFabricaSourceFlag(t *testing.T) {
	cmd := newGenerateCommand()
	flag := cmd.Flags().Lookup("fabrica-source")
	if flag == nil {
		t.Fatal("expected generate command to expose --fabrica-source")
	}
}

func TestResolveGenerateFabricaSource_FromEnv(t *testing.T) {
	t.Setenv("FABRICA_SOURCE_PATH", "")

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "go.mod"), []byte("module github.com/openchami/fabrica\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("FABRICA_SOURCE_PATH", sourceDir)

	got, err := resolveGenerateFabricaSource("")
	if err != nil {
		t.Fatalf("resolveGenerateFabricaSource: %v", err)
	}

	want, err := filepath.Abs(sourceDir)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if got != want {
		t.Fatalf("resolved source = %q, want %q", got, want)
	}
}

func TestGenerateIsolatedRunnerGoModIncludesReplaces(t *testing.T) {
	projectRoot := "/tmp/project"
	fabricaSource := "/tmp/fabrica"
	modulePath := "example.com/demo"

	got := generateIsolatedRunnerGoMod(modulePath, projectRoot, fabricaSource, "1.24.0")

	for _, want := range []string{
		"module fabrica-codegen-runner",
		"go 1.24.0",
		"github.com/openchami/fabrica v0.0.0",
		"example.com/demo v0.0.0",
		"replace github.com/openchami/fabrica => /tmp/fabrica",
		"replace example.com/demo => /tmp/project",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated go.mod missing %q:\n%s", want, got)
		}
	}
}

func TestLocalModulePlaceholderVersionSupportsMajorSuffix(t *testing.T) {
	if got := localModulePlaceholderVersion("example.com/demo"); got != "v0.0.0" {
		t.Fatalf("localModulePlaceholderVersion() = %q, want v0.0.0", got)
	}

	if got := localModulePlaceholderVersion("example.com/demo/v2"); got != "v2.0.0" {
		t.Fatalf("localModulePlaceholderVersion(/v2) = %q, want v2.0.0", got)
	}
}

func TestDetectProjectGoVersion(t *testing.T) {
	projectRoot := t.TempDir()
	goMod := "module example.com/demo\n\ngo 1.24.3\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if got := detectProjectGoVersion(projectRoot); got != "1.24.3" {
		t.Fatalf("detectProjectGoVersion() = %q, want 1.24.3", got)
	}
}

func TestGenerateRunnerCodeUsesProjectRoot(t *testing.T) {
	got := generateRunnerCode("/tmp/project", "example.com/demo", "cmd/server", "main", true, false, false, false, false, "file")

	if !strings.Contains(got, "os.Chdir(\"/tmp/project\")") {
		t.Fatalf("runner code missing project root chdir:\n%s", got)
	}
	if !strings.Contains(got, "gen.GenerateMiddleware()") {
		t.Fatalf("runner code missing middleware generation:\n%s", got)
	}
}
