// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedAnnotationProject_selects_mixed_storage_for_multiple_resources(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeAPIsResources(t, "Token", "Device")
	project.writeNamedResourceSource(t, "device_types.go", generatedDeviceSource)

	// When
	project.generate(t)

	// Then
	for _, artifact := range []string{
		filepath.Join("internal", "storage", "ent", "schema", "resource.go"),
		filepath.Join("internal", "storage", "ent", "schema", "token.go"),
		filepath.Join("internal", "storage", "ent_adapter_token.go"),
	} {
		if _, err := os.Stat(filepath.Join(project.root, artifact)); err != nil {
			t.Errorf("mixed generation missing %s: %v", artifact, err)
		}
	}
	for _, artifact := range []string{
		filepath.Join("internal", "storage", "ent", "schema", "device.go"),
		filepath.Join("internal", "storage", "ent_adapter_device.go"),
	} {
		if _, err := os.Stat(filepath.Join(project.root, artifact)); !os.IsNotExist(err) {
			t.Errorf("unannotated Device selected dedicated artifact %s: %v", artifact, err)
		}
	}
}

func TestGeneratedAnnotationProject_joins_invalid_resources_before_output_updates(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeAPIsResources(t, "Token", "Device")
	project.writeResourceSource(t, generatedUnannotatedTokenSource)
	project.writeNamedResourceSource(t, "device_types.go", generatedDeviceSource)
	firstResult := project.generate(t)
	if firstResult.err != nil {
		t.Fatalf("prepare generic generated output: %s", firstResult.failureMessage())
	}
	dedicatedSentinel := filepath.Join(project.root, "internal", "storage", "ent", "schema", "token.go")
	if err := os.WriteFile(dedicatedSentinel, []byte("dedicated-sentinel\n"), 0o600); err != nil {
		t.Fatalf("write dedicated sentinel: %v", err)
	}
	tracked := []string{
		filepath.Join("pkg", "resources", "register_generated.go"),
		filepath.Join("internal", "storage", "ent", "schema", "resource.go"),
		filepath.Join("internal", "storage", "ent", "schema", "label.go"),
		filepath.Join("internal", "storage", "ent", "schema", "annotation.go"),
		filepath.Join("internal", "storage", "ent_adapter.go"),
		filepath.Join("internal", "storage", "ent", "schema", "token.go"),
	}
	before := project.readArtifacts(t, tracked)
	project.writeResourceSource(t, malformedAnnotatedTokenSource)
	project.writeNamedResourceSource(
		t,
		"device_types.go",
		strings.ReplaceAll(malformedAnnotatedTokenSource, "Token", "Device"),
	)

	// When
	result := project.generate(t)

	// Then
	if result.err == nil {
		t.Fatalf("stage %q succeeded for two malformed resources", result.stage)
	}
	combined := result.stdout + result.stderr
	for _, expected := range []string{"token_types.go", "device_types.go", "sensitve"} {
		if !strings.Contains(combined, expected) {
			t.Errorf("joined failure missing %q\n%s", expected, result.failureMessage())
		}
	}
	after := project.readArtifacts(t, tracked)
	for path, want := range before {
		if got := after[path]; got != want {
			t.Errorf("validation failure partially updated %s", path)
		}
	}
}

func (p generatedProject) writeAPIsResources(t *testing.T, resources ...string) {
	t.Helper()
	content := "groups:\n  - name: acceptance.example.io\n    storageVersion: v1\n    versions: [v1]\n    resources: [" + strings.Join(resources, ", ") + "]\n"
	if err := os.WriteFile(filepath.Join(p.root, "apis.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write apis.yaml: %v", err)
	}
}

func (p generatedProject) writeNamedResourceSource(t *testing.T, filename, source string) {
	t.Helper()
	path := filepath.Join(p.root, "apis", "acceptance.example.io", "v1", filename)
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write resource source %s: %v", filename, err)
	}
}

func (p generatedProject) readArtifacts(t *testing.T, paths []string) map[string]string {
	t.Helper()
	artifacts := make(map[string]string, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(p.root, path))
		if err != nil {
			t.Fatalf("read generated artifact %s: %v", path, err)
		}
		artifacts[path] = string(content)
	}
	return artifacts
}

const generatedUnannotatedTokenSource = `package v1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
)

type Token struct {
	APIVersion string
	Kind string
	Metadata fabrica.Metadata
	Spec TokenSpec
	Status TokenStatus
}
type TokenSpec struct { Value string }
type TokenStatus struct{}
func (r *Token) Validate(context.Context) error { return nil }
func (r *Token) GetKind() string { return "Token" }
func (r *Token) GetName() string { return r.Metadata.Name }
func (r *Token) GetUID() string { return r.Metadata.UID }
func (r *Token) IsHub() {}`

const generatedDeviceSource = `package v1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
)

type Device struct {
	APIVersion string
	Kind string
	Metadata fabrica.Metadata
	Spec DeviceSpec
	Status DeviceStatus
}
type DeviceSpec struct { Name string }
type DeviceStatus struct{}
func (r *Device) Validate(context.Context) error { return nil }
func (r *Device) GetKind() string { return "Device" }
func (r *Device) GetName() string { return r.Metadata.Name }
func (r *Device) GetUID() string { return r.Metadata.UID }
func (r *Device) IsHub() {}`
