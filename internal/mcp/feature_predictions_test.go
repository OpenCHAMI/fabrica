// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"os"
	"path/filepath"
	"slices"
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
