// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaultsMissingGeneration(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`
project:
  name: test
  module: example.com/test
features:
  validation:
    enabled: true
    mode: strict
  events:
    enabled: false
  conditional:
    enabled: true
  auth:
    enabled: false
  storage:
    enabled: true
    type: file
`)
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), content, 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if !cfg.Generation.Handlers || !cfg.Generation.Storage || !cfg.Generation.Client || !cfg.Generation.OpenAPI || !cfg.Generation.Middleware {
		t.Fatalf("Generation defaults not applied: %+v", cfg.Generation)
	}
	if cfg.Generation.Events || cfg.Generation.Reconciliation {
		t.Fatalf("expected events and reconciliation generation to default off: %+v", cfg.Generation)
	}
}

func TestLoadConfigGenerationExplicitFalseOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`
project:
  name: test
  module: example.com/test
features:
  storage:
    enabled: true
    type: file
generation:
  handlers: false
  storage: false
  client: false
  openapi: false
  middleware: false
`)
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), content, 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Generation.Handlers || cfg.Generation.Storage || cfg.Generation.Client || cfg.Generation.OpenAPI || cfg.Generation.Middleware {
		t.Fatalf("explicit false generation settings were not preserved: %+v", cfg.Generation)
	}
}

func TestLoadConfigDefaultsMissingStorageEnabled(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`
project:
  name: test
  module: example.com/test
features:
  storage:
    type: file
`)
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), content, 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}
	if !cfg.Features.Storage.Enabled {
		t.Fatal("storage should default to enabled when features.storage.enabled is omitted")
	}
}

func TestLoadConfigStorageExplicitFalseOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`
project:
  name: test
  module: example.com/test
features:
  storage:
    enabled: false
    type: file
`)
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), content, 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}
	if cfg.Features.Storage.Enabled {
		t.Fatal("explicit features.storage.enabled=false should override the default")
	}
}
