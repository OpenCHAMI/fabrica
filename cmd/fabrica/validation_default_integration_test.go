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
	"testing"
	"time"

	"github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/internal/projectfeatures"
	"github.com/openchami/fabrica/pkg/codegen"
	"gopkg.in/yaml.v3"
)

// The runner must write the exact normalized selection checked by preflight,
// including omitted keys, aliases, and overlapping legacy/security settings.
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

	for _, scenario := range []struct {
		name string
		edit func(validation map[string]any, features map[string]any)
	}{
		{"omitted-enabled", func(validation, _ map[string]any) { delete(validation, "enabled") }},
		{"omitted-block", func(_, features map[string]any) { delete(features, "validation") }},
		{"disabled-overrides-warn", func(validation, features map[string]any) {
			validation["enabled"] = false
			validation["mode"] = "warn"
			features["conditional"] = map[string]any{"enabled": true, "etag_algorithm": "md5"}
			features["storage"].(map[string]any)["db_driver"] = "sqlite"
		}},
		{"legacy-auth-and-workers", func(validation, features map[string]any) {
			validation["mode"] = "warn"
			features["auth"] = map[string]any{"enabled": true}
			features["security"] = map[string]any{"authn": map[string]any{"enabled": false}, "authz": map[string]any{"enabled": false, "mode": "shadow"}}
			features["events"] = map[string]any{"enabled": true, "bus_type": "memory"}
			features["reconciliation"] = map[string]any{"enabled": true, "worker_count": 2, "requeue_delay": 7}
			features["metrics"] = map[string]any{"enabled": true}
		}},
		{"ent-sqlite-alias", func(_, features map[string]any) {
			features["storage"] = map[string]any{"enabled": true, "type": "ent", "db_driver": "sqlite"}
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "svc")
			run(t, filepath.Dir(dir), binary, "init", "svc", "--module", "example.com/svc", "--storage-type", "file")
			run(t, dir, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo)
			run(t, dir, binary, "add", "resource", "Node")

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

			// Explicit storage selection must win even over misleading legacy comments.
			mainPath := filepath.Join(dir, "cmd/server/main.go")
			mainSource, err := os.ReadFile(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			mainSource = append(mainSource, []byte("\n// legacy internal/storage/ent reference\n")...)
			if err := os.WriteFile(mainPath, mainSource, 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := config.ValidateConfig(cfg); err != nil {
				t.Fatal(err)
			}
			want := projectfeatures.FromConfig(cfg, true)
			run(t, dir, binary, "generate", "--fabrica-source", repo)
			var manifest struct{ Config codegen.FeatureConfig }

			data, err = os.ReadFile(filepath.Join(dir, "cmd/server/features_manifest_generated.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.Config != want {
				t.Fatalf("manifest = %#v\npreflight = %#v", manifest.Config, want)
			}
			after, err := os.ReadFile(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(mainSource) {
				t.Fatal("generation rewrote user main.go")
			}

			// The CLI's own reading must match the recorded selection.
			run(t, dir, binary, "generate", "--handlers", "--fabrica-source", repo)
		})
	}
}
