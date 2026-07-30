// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/openchami/fabrica/internal/config"
)

func TestDiscoverVersionedResources_absent_hub_keeps_configured_resource_generic(t *testing.T) {
	// Given
	t.Chdir(t.TempDir())
	apisConfig := configpkg.DefaultAPIsConfig("test.example", "v1", []string{"v1"})
	apisConfig.Groups[0].Resources = []string{"Widget"}

	// When
	resources, err := discoverVersionedResources(apisConfig)

	// Then
	if err != nil {
		t.Fatalf("discoverVersionedResources() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources = %#v, want one configured resource", resources)
	}
	if resources[0].SourcePath != "" {
		t.Fatalf("configured resource SourcePath = %q, want empty", resources[0].SourcePath)
	}
	registration := generateVersionedRegistrationCode("example.com/test", apisConfig, resources)
	if !strings.Contains(registration, "gen.RegisterResource(&v1.Widget{})") {
		t.Fatalf("source-less registration did not remain generic\n%s", registration)
	}
	if strings.Contains(registration, "RegisterResourceFromSource") {
		t.Fatalf("source-less registration used source-aware API\n%s", registration)
	}
}

func TestDiscoverVersionedResources_preserves_nonconventional_walked_filename(t *testing.T) {
	// Given
	root := t.TempDir()
	t.Chdir(root)
	apisConfig := configpkg.DefaultAPIsConfig("test.example", "v1", []string{"v1"})
	hubDir := filepath.Join("apis", "test.example", "v1")
	if err := os.MkdirAll(hubDir, 0o755); err != nil {
		t.Fatalf("create hub directory: %v", err)
	}
	sourcePath := filepath.Join(hubDir, "inventory.go")
	const source = `package v1

type Widget struct {
	APIVersion string
	Kind string
	Metadata struct{}
}`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write non-conventional resource file: %v", err)
	}

	// When
	resources, err := discoverVersionedResources(apisConfig)

	// Then
	if err != nil {
		t.Fatalf("discoverVersionedResources() error = %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources = %#v, want walked Widget", resources)
	}
	if resources[0].Name != "Widget" || resources[0].SourcePath != sourcePath {
		t.Fatalf("discovered resource = %#v, want exact walked path %q", resources[0], sourcePath)
	}
}
