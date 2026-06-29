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

func TestAddYAMLTagToStructTag(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		changed bool
	}{
		{
			name:    "adds yaml after json",
			in:      "`json:\"name,omitempty\" validate:\"required\"`",
			want:    "`json:\"name,omitempty\" yaml:\"name,omitempty\" validate:\"required\"`",
			changed: true,
		},
		{
			name:    "preserves existing yaml",
			in:      "`json:\"name\" yaml:\"custom\" validate:\"required\"`",
			want:    "`json:\"name\" yaml:\"custom\" validate:\"required\"`",
			changed: false,
		},
		{
			name:    "skips ignored json fields",
			in:      "`json:\"-\"`",
			want:    "`json:\"-\"`",
			changed: false,
		},
		{
			name:    "skips tags without json",
			in:      "`validate:\"required\"`",
			want:    "`validate:\"required\"`",
			changed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := addYAMLTagToStructTag(tt.in)
			if got != tt.want || changed != tt.changed {
				t.Fatalf("addYAMLTagToStructTag() = %q, %v; want %q, %v", got, changed, tt.want, tt.changed)
			}
		})
	}
}

func TestMigrateYAMLTagsDryRunAndWrite(t *testing.T) {
	root := t.TempDir()
	versionDir := filepath.Join(root, "apis", "example.fabrica.dev", "v1")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	apisYAML := `groups:
  - name: example.fabrica.dev
    storageVersion: v1
    versions:
      - v1
    resources:
      - Device
`
	if err := os.WriteFile(filepath.Join(root, "apis.yaml"), []byte(apisYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(apis.yaml): %v", err)
	}

	typePath := filepath.Join(versionDir, "device_types.go")
	original := `package v1

type Device struct {
	APIVersion string ` + "`json:\"apiVersion\"`" + `
	Kind string ` + "`json:\"kind\" yaml:\"kind\"`" + `
	Spec DeviceSpec ` + "`json:\"spec\" validate:\"required\"`" + `
	Internal string ` + "`json:\"-\"`" + `
}

type DeviceSpec struct {
	Name string ` + "`json:\"name,omitempty\" validate:\"required\"`" + `
}
`
	if err := os.WriteFile(typePath, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile(device_types.go): %v", err)
	}

	dryRunResult, err := migrateYAMLTags(root, true)
	if err != nil {
		t.Fatalf("migrateYAMLTags dry-run: %v", err)
	}
	if dryRunResult.FilesScanned != 1 || dryRunResult.FilesChanged != 1 || dryRunResult.TagsAdded != 3 {
		t.Fatalf("dry-run result = %+v, want 1 scanned, 1 changed, 3 tags", dryRunResult)
	}
	data, err := os.ReadFile(typePath)
	if err != nil {
		t.Fatalf("ReadFile after dry-run: %v", err)
	}
	if string(data) != original {
		t.Fatalf("dry-run modified file")
	}

	writeResult, err := migrateYAMLTags(root, false)
	if err != nil {
		t.Fatalf("migrateYAMLTags write: %v", err)
	}
	if writeResult.FilesScanned != 1 || writeResult.FilesChanged != 1 || writeResult.TagsAdded != 3 {
		t.Fatalf("write result = %+v, want 1 scanned, 1 changed, 3 tags", writeResult)
	}
	updatedBytes, err := os.ReadFile(typePath)
	if err != nil {
		t.Fatalf("ReadFile after write: %v", err)
	}
	updated := string(updatedBytes)
	for _, want := range []string{
		"APIVersion string     `json:\"apiVersion\" yaml:\"apiVersion\"`",
		"Kind       string     `json:\"kind\" yaml:\"kind\"`",
		"Spec       DeviceSpec `json:\"spec\" yaml:\"spec\" validate:\"required\"`",
		"Internal   string     `json:\"-\"`",
		"Name string `json:\"name,omitempty\" yaml:\"name,omitempty\" validate:\"required\"`",
	} {
		if !strings.Contains(updated, want) {
			t.Fatalf("updated file missing %q:\n%s", want, updated)
		}
	}
}

func TestGenerateResourceFileEmitsYAMLTags(t *testing.T) {
	root := t.TempDir()
	versionedPath := filepath.Join(root, "device_types.go")
	opts := &addOptions{withValidation: true, withStatus: true, withVersioning: true, version: "v1"}
	if err := generateResourceFile(versionedPath, "Device", true, opts, "github.com/example/test", "v1", "example.fabrica.dev"); err != nil {
		t.Fatalf("generateResourceFile(versioned): %v", err)
	}
	versioned := mustReadPath(t, versionedPath)
	for _, want := range []string{
		"APIVersion string           `json:\"apiVersion\" yaml:\"apiVersion\"`",
		"Spec       DeviceSpec       `json:\"spec\" yaml:\"spec\" validate:\"required\"`",
		"Status     DeviceStatus     `json:\"status,omitempty\" yaml:\"status,omitempty\"`",
		"Description string `json:\"description,omitempty\" yaml:\"description,omitempty\" validate:\"max=200\"`",
		"Version string `json:\"version,omitempty\" yaml:\"version,omitempty\"`",
	} {
		if !strings.Contains(versioned, want) {
			t.Fatalf("versioned resource missing %q:\n%s", want, versioned)
		}
	}

	legacyPath := filepath.Join(root, "legacy_types.go")
	legacyOpts := &addOptions{withValidation: true, withStatus: true, packageName: "device"}
	if err := generateResourceFile(legacyPath, "Device", false, legacyOpts, "", "", ""); err != nil {
		t.Fatalf("generateResourceFile(legacy): %v", err)
	}
	legacy := mustReadPath(t, legacyPath)
	for _, want := range []string{
		"Spec   DeviceSpec   `json:\"spec\" yaml:\"spec\" validate:\"required\"`",
		"Status DeviceStatus `json:\"status,omitempty\" yaml:\"status,omitempty\"`",
	} {
		if !strings.Contains(legacy, want) {
			t.Fatalf("legacy resource missing %q:\n%s", want, legacy)
		}
	}
}

func TestResourceTemplatesEmitYAMLTags(t *testing.T) {
	checks := map[string][]string{
		"pkg/codegen/templates/server/models.go.tmpl": {
			"`json:\"metadata\" yaml:\"metadata\" validate:\"required\"`",
			"`json:\"spec,omitempty\" yaml:\"spec,omitempty\" validate:\"omitempty\"`",
		},
		"pkg/codegen/templates/client/models.go.tmpl": {
			"`json:\"metadata\" yaml:\"metadata\" validate:\"required\"`",
			"`json:\"spec,omitempty\" yaml:\"spec,omitempty\" validate:\"omitempty\"`",
		},
		"pkg/codegen/templates/client/client.go.tmpl": {
			"`json:\"versionId\" yaml:\"versionId\"`",
			"`json:\"spec\" yaml:\"spec\"`",
		},
		"pkg/codegen/templates/storage/plugins.go.tmpl": {
			"`json:\"versionId\" yaml:\"versionId\"`",
			"`json:\"spec\" yaml:\"spec\"`",
		},
	}

	for file, needles := range checks {
		t.Run(file, func(t *testing.T) {
			content := mustReadFile(t, file)
			for _, needle := range needles {
				if !strings.Contains(content, needle) {
					t.Fatalf("%s missing %q", file, needle)
				}
			}
		})
	}
}

func mustReadPath(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(data)
}
