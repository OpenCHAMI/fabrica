// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// The CLI preflight and the generated runner read .fabrica.yaml separately.
// Omitted validation keys must resolve to strict in both, or handlers silently
// drop validation and the manifest disagrees with every later selective run.
func TestGeneratedValidationDefaultsMatchCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("generated service integration")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	binary := filepath.Join(workspace, "fabrica")
	run := func(t *testing.T, dir, executable string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", executable, args, err, out)
		}
	}
	run(t, repo, "go", "build", "-o", binary, "./cmd/fabrica")
	dir := filepath.Join(workspace, "svc")
	run(t, workspace, binary, "init", "svc", "--module", "example.com/svc", "--storage-type", "file")
	run(t, dir, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo)
	run(t, dir, binary, "add", "resource", "Node")

	for _, scenario := range []struct {
		name string
		edit func(validation map[string]any, features map[string]any)
	}{
		{"omitted-enabled", func(validation, _ map[string]any) { delete(validation, "enabled") }},
		{"omitted-block", func(_, features map[string]any) { delete(features, "validation") }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(dir, ".fabrica.yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := yaml.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			features := doc["features"].(map[string]any)
			validation, _ := features["validation"].(map[string]any)
			if validation == nil {
				validation = map[string]any{"mode": "strict"}
				features["validation"] = validation
			}
			validation["enabled"] = true
			validation["mode"] = "strict"
			scenario.edit(validation, features)
			if data, err = yaml.Marshal(doc); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}

			run(t, dir, binary, "generate", "--fabrica-source", repo)
			handlers, err := filepath.Glob(filepath.Join(dir, "cmd/server/*handlers*_generated.go"))
			if err != nil || len(handlers) == 0 {
				t.Fatalf("handlers: %v, %v", handlers, err)
			}
			for _, handler := range handlers {
				source, err := os.ReadFile(handler)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(source), `validation.Mode("disabled")`) || !strings.Contains(string(source), `validation.Mode("strict")`) {
					t.Fatalf("%s does not use strict validation", handler)
				}
			}
			var manifest struct {
				Config struct{ ValidationMode string }
			}
			data, err = os.ReadFile(filepath.Join(dir, "cmd/server/features_manifest_generated.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.Config.ValidationMode != "strict" {
				t.Fatalf("manifest validation mode = %q", manifest.Config.ValidationMode)
			}
			if _, err := os.Stat(filepath.Join(dir, "internal/middleware/validation_middleware_generated.go")); err != nil {
				t.Fatalf("validation middleware: %v", err)
			}
			// The CLI's own reading must match the recorded selection.
			run(t, dir, binary, "generate", "--handlers", "--fabrica-source", repo)
		})
	}
}
