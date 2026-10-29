// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	configpkg "github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/internal/projectfeatures"
	"github.com/openchami/fabrica/pkg/codegen"
)

func TestPreviewFeatureGenerationReportsRemovalsAndChecksSelection(t *testing.T) {
	root := t.TempDir()
	server := filepath.Join(root, "cmd/server")
	if err := os.MkdirAll(server, 0755); err != nil {
		t.Fatal(err)
	}
	main := []byte("package main\ntype Config struct{GeneratedFeatureConfig};func start(){loadGeneratedFeatures(nil,nil)}")
	if err := os.WriteFile(filepath.Join(server, "main.go"), main, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := configpkg.NewDefaultConfig("fixture", "example.com/fixture")
	cfg.Features.Metrics.Enabled = true
	if err := configpkg.SaveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	gen := codegen.NewGenerator(server, "main", cfg.Project.Module)
	gen.SetFeatures(projectfeatures.FromConfig(cfg, false))
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	cfg.Features.Metrics.Enabled = false
	if err := configpkg.SaveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	writes, removals, err := previewFeatureGeneration(root, []string{"--debug"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(writes, "cmd/server/features_generated.go") || !slices.Contains(writes, "cmd/server/features_manifest_generated.json") {
		t.Fatalf("writes=%v", writes)
	}
	if !slices.Contains(removals, "cmd/server/metrics_generated.go") {
		t.Fatalf("removals=%v", removals)
	}
	if _, _, err := previewFeatureGeneration(root, []string{"--handlers"}); err == nil {
		t.Fatal("selective change accepted")
	}
	if _, err := os.Stat(filepath.Join(server, "metrics_generated.go")); err != nil {
		t.Fatal("preview mutated metrics output")
	}
}

func TestMCPFeatureRecoveryPreviewIsReadOnly(t *testing.T) {
	root := t.TempDir()
	server := filepath.Join(root, "cmd/server")
	if err := os.MkdirAll(server, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(server, "main.go"), []byte("package main\ntype Config struct{GeneratedFeatureConfig};func start(){loadGeneratedFeatures(nil,nil)}"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := configpkg.NewDefaultConfig("fixture", "example.com/fixture")
	cfg.Features.Metrics.Enabled = true
	if err := configpkg.SaveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	gen := codegen.NewGenerator(server, "main", cfg.Project.Module)
	gen.SetFeatures(projectfeatures.FromConfig(cfg, false))
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	// Model interruption after the manifest commit, before journal cleanup. These
	// bytes come from the actual generated outputs, not hand-written templates.
	change := func(path string) map[string]interface{} {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		state := map[string]interface{}{"Exists": false, "Digest": ""}
		if err == nil {
			sum := sha256.Sum256(data)
			state = map[string]interface{}{"Exists": true, "Digest": hex.EncodeToString(sum[:])}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return map[string]interface{}{"Path": path, "Before": state, "After": state, "Data": data, "Mode": 0644}
	}
	var files []map[string]interface{}
	for _, path := range []string{
		"cmd/server/features_generated.go", "cmd/server/runtime_helpers_generated.go", "cmd/server/auth_helpers_generated.go",
		"cmd/server/metrics_generated.go", "cmd/server/metrics_helpers_generated.go",
		"internal/middleware/validation_middleware_generated.go", "internal/middleware/conditional_middleware_generated.go",
		"internal/middleware/versioning_middleware_generated.go", "internal/middleware/event_bus_generated.go",
	} {
		files = append(files, change(path))
	}
	journal := map[string]interface{}{"Format": 1, "Config": gen.Config.Features, "Files": files, "Manifest": change("cmd/server/features_manifest_generated.json")}
	encoded, err := json.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(server, ".fabrica-features-transaction.json")
	if err := os.WriteFile(journalPath, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Features.Metrics.Enabled = false
	if err := configpkg.SaveConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := make(map[string]string)
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			files[path] = string(data) + info.ModTime().String()
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return files
	}
	srv := &mcpServer{workspaceRoot: root}
	for _, conflict := range []bool{false, true} {
		if conflict {
			if err := os.WriteFile(filepath.Join(server, "metrics_generated.go"), []byte("// user customization"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		before := snapshot()
		for _, artifacts := range [][]interface{}{nil, {"handlers"}, {"client"}} {
			result, err := srv.toolGenerateCode(map[string]interface{}{"project_path": root, "mode": "dry_run", "artifacts": artifacts})
			if err != nil {
				t.Fatal(err)
			}
			if result["status"] != "recovery_required" || !strings.Contains(result["next_action"].(string), "full 'fabrica generate'") {
				t.Fatalf("missing recovery action: %+v", result)
			}
			if !slices.Contains(result["planned_files"].([]string), "cmd/server/metrics_generated.go") {
				t.Fatalf("missing affected files: %+v", result)
			}
			if _, ok := result["recovery_conflict"]; ok != conflict {
				t.Fatalf("conflict=%t: %+v", conflict, result)
			}
		}
		if !reflect.DeepEqual(before, snapshot()) {
			t.Fatal("dry run changed files, lock, or recovery evidence")
		}
	}
	if err := os.WriteFile(journalPath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	before := snapshot()
	if _, err := srv.toolGenerateCode(map[string]interface{}{"project_path": root, "mode": "dry_run"}); err == nil {
		t.Fatal("corrupt journal accepted")
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("corrupt journal preview mutated files")
	}
}
