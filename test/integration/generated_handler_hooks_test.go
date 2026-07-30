// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedHandlerHooks_runtime_behavior(t *testing.T) {
	project := newGeneratedProject(t, "file")
	project.writeResourceSource(t, generatedUnannotatedTokenSource)
	if err := os.WriteFile(
		filepath.Join(project.root, "cmd", "server", "main.go"),
		[]byte("package main\n\ntype Config struct{}\n\nfunc main() {}\n"),
		0o644,
	); err != nil {
		t.Fatalf("write server fixture: %v", err)
	}
	result := project.run(
		t,
		"generate-handler-hooks",
		project.root,
		project.fabricaBin,
		"generate",
		"--force",
		"--debug",
		"--fabrica-source",
		project.repoRoot,
	)
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	runtimePath := filepath.Join(project.root, "cmd", "server", "generated_handler_hooks_test.go")
	if err := os.WriteFile(runtimePath, []byte(generatedHandlerHooksRuntimeTest), 0o644); err != nil {
		t.Fatalf("write generated handler hook runtime test: %v", err)
	}
	if result := project.tidy(t); result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	result = project.run(t, "generated-handler-hooks-runtime", project.root, "go", "test", "-count=1", "-v", "./cmd/server")
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	if !strings.Contains(result.stdout, "generated handler hook runtime") {
		t.Fatalf("runtime output missing handler hook receipt\n%s", result.stdout)
	}
}

func TestGeneratedHandlerHooks_create_once_file_survives_force_regeneration(t *testing.T) {
	project := newGeneratedProject(t, "file")
	project.writeResourceSource(t, generatedUnannotatedTokenSource)
	result := project.run(
		t,
		"generate-handler-hook-stub",
		project.root,
		project.fabricaBin,
		"generate",
		"--force",
		"--debug",
		"--fabrica-source",
		project.repoRoot,
	)
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}

	hookPath := filepath.Join(project.root, "cmd", "server", "token_hooks.go")
	custom := []byte("package main\n\n// retained user hook setup\nvar tokenHooks = TokenHooks{}\n")
	if err := os.WriteFile(hookPath, custom, 0o644); err != nil {
		t.Fatalf("customize handler hook stub: %v", err)
	}
	result = project.run(
		t,
		"force-regenerate-handler-hooks",
		project.root,
		project.fabricaBin,
		"generate",
		"--force",
		"--debug",
		"--fabrica-source",
		project.repoRoot,
	)
	if result.err != nil {
		t.Fatalf("%s", result.failureMessage())
	}
	preserved, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read preserved handler hook stub: %v", err)
	}
	if string(preserved) != string(custom) {
		t.Fatalf("handler hook stub was overwritten:\n%s", preserved)
	}
}
