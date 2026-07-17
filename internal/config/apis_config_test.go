// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAPIsConfigResourceValidation(t *testing.T) {
	cfg := &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           "example.openchami.io",
				StorageVersion: "v1",
				Versions:       []string{"v1"},
				Resources: APIResources{
					{
						Name:       "Console",
						Path:       "/remote-console/consoles",
						Operations: []string{"list"},
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
}

func TestAPIsConfigResourceRejectsUnknownOperation(t *testing.T) {
	cfg := &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           "example.openchami.io",
				StorageVersion: "v1",
				Versions:       []string{"v1"},
				Resources: APIResources{
					{
						Name:       "Console",
						Operations: []string{"explode"},
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected Validate() to reject unknown operation")
	}
}

func TestAPIsConfigResourceRejectsInvalidPath(t *testing.T) {
	cfg := &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           "example.openchami.io",
				StorageVersion: "v1",
				Versions:       []string{"v1"},
				Resources: APIResources{
					{
						Name: "Console",
						Path: "remote-console/consoles",
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected Validate() to reject path without leading slash")
	}
}

func TestAPIsConfigResourceRejectsNonCanonicalOrUnsafePath(t *testing.T) {
	for name, resourcePath := range map[string]string{
		"surrounding whitespace": " /devices",
		"trailing slash":         "/devices/",
		"empty segment":          "/hardware//devices",
		"current segment":        "/hardware/./devices",
		"parent segment":         "/hardware/../devices",
		"query":                  "/devices?active=true",
		"fragment":               "/devices#active",
		"percent escape":         "/hardware%2Fdevices",
		"backslash":              `/hardware\devices`,
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &APIsConfig{
				Groups: []APIGroup{
					{
						Name:           "example.openchami.io",
						StorageVersion: "v1",
						Versions:       []string{"v1"},
						Resources: APIResources{
							{Name: "Device", Path: resourcePath},
						},
					},
				},
			}

			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected path %q to be rejected", resourcePath)
			}
		})
	}
}

func TestAPIsConfigResourceAcceptsCanonicalPath(t *testing.T) {
	cfg := &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           "example.openchami.io",
				StorageVersion: "v1",
				Versions:       []string{"v1"},
				Resources: APIResources{
					{Name: "Device", Path: "/hardware-v2/device_types/~active"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected canonical path to be accepted: %v", err)
	}
}

func TestAPIsConfigResourceRejectsReservedPath(t *testing.T) {
	cfg := &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           "example.openchami.io",
				StorageVersion: "v1",
				Versions:       []string{"v1"},
				Resources: APIResources{
					{
						Name: "Console",
						Path: "/openapi.json",
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected Validate() to reject reserved path")
	}
}

func TestAPIsConfigResourceRejectsDuplicatePath(t *testing.T) {
	cfg := &APIsConfig{
		Groups: []APIGroup{
			{
				Name:           "example.openchami.io",
				StorageVersion: "v1",
				Versions:       []string{"v1"},
				Resources: APIResources{
					{
						Name: "Console",
						Path: "/resources",
					},
					{
						Name: "Device",
						Path: "/resources",
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected Validate() to reject duplicate path")
	}
}

func TestAPIsConfigResourceRejectsGeneratedRouteCollision(t *testing.T) {
	for name, conflictingPath := range map[string]string{
		"item route":   "/devices/metrics",
		"status route": "/devices/example/status",
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &APIsConfig{
				Groups: []APIGroup{
					{
						Name:           "example.openchami.io",
						StorageVersion: "v1",
						Versions:       []string{"v1"},
						Resources: APIResources{
							{Name: "Device", Path: "/devices"},
							{Name: "Metric", Path: conflictingPath},
						},
					},
				},
			}

			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected generated route collision for %q", conflictingPath)
			}
		})
	}
}

func TestAPIsConfigResourcesAcceptsListSyntax(t *testing.T) {
	var cfg APIsConfig
	if err := yaml.Unmarshal([]byte(`
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions:
      - v1
    resources:
      - Console
      - Device
`), &cfg); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	group := cfg.Groups[0]
	if got, want := group.Resources.Names(), []string{"Console", "Device"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Resources.Names() = %v, want %v", got, want)
	}
}

func TestAPIsConfigResourcesRejectsConfiguredListEntries(t *testing.T) {
	for name, input := range map[string]string{
		"single-entry map": `
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions: [v1]
    resources:
      - Console:
          operations: [list]
`,
		"named object": `
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions: [v1]
    resources:
      - name: Console
        operations: [list]
`,
	} {
		t.Run(name, func(t *testing.T) {
			var cfg APIsConfig
			err := yaml.Unmarshal([]byte(input), &cfg)
			if err == nil {
				t.Fatal("expected configured list entry to be rejected")
			}
			if !strings.Contains(err.Error(), "use map syntax for resource configuration") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAPIsConfigResourcesAcceptsMapSyntax(t *testing.T) {
	var cfg APIsConfig
	if err := yaml.Unmarshal([]byte(`
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions:
      - v1
    resources:
      Console:
        path: /remote-console/consoles
        operations:
          - list
          - get
`), &cfg); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	resource, ok := cfg.Groups[0].Resources.Get("Console")
	if !ok {
		t.Fatalf("expected Console resource")
	}
	if resource.Path != "/remote-console/consoles" {
		t.Fatalf("Path = %q, want %q", resource.Path, "/remote-console/consoles")
	}
	if got, want := resource.Operations, []string{"list", "get"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Operations = %v, want %v", got, want)
	}
}

func TestAPIsConfigResourcesRejectsEmptyOperations(t *testing.T) {
	var cfg APIsConfig
	err := yaml.Unmarshal([]byte(`
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions:
      - v1
    resources:
      Console:
        operations: []
`), &cfg)
	if err == nil {
		t.Fatal("expected empty operations to be rejected")
	}
	if !strings.Contains(err.Error(), "resources.Console.operations must contain at least one operation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIsConfigResourcesAllowsOmittedOperations(t *testing.T) {
	var cfg APIsConfig
	if err := yaml.Unmarshal([]byte(`
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions:
      - v1
    resources:
      Console:
        path: /remote-console/consoles
`), &cfg); err != nil {
		t.Fatalf("omitted operations should use defaults: %v", err)
	}

	resource, ok := cfg.Groups[0].Resources.Get("Console")
	if !ok {
		t.Fatal("expected Console resource")
	}
	if resource.Operations != nil {
		t.Fatalf("Operations = %v, want nil defaults", resource.Operations)
	}
}

func TestAPIsConfigResourcesRejectsUnknownField(t *testing.T) {
	var cfg APIsConfig
	err := yaml.Unmarshal([]byte(`
groups:
  - name: example.openchami.io
    storageVersion: v1
    versions:
      - v1
    resources:
      Console:
        operatons:
          - list
`), &cfg)
	if err == nil {
		t.Fatal("expected unknown resource field to be rejected")
	}
	if !strings.Contains(err.Error(), `resources.Console contains unsupported field "operatons"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
