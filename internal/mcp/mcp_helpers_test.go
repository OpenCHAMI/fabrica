// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"os"
	"path/filepath"
	"testing"

	configpkg "github.com/openchami/fabrica/internal/config"
)

func TestInspectDiscoveredEndpoints(t *testing.T) {
	for name, resources := range map[string]string{"absent resource list": "", "stale resource list": "    resources: [Gadget]\n"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				".fabrica.yaml": "{}\n",
				"apis.yaml": "groups:\n  - name: example.dev\n" +
					"    storageVersion: v1\n    versions: [v1]\n" + resources,
				"apis/example.dev/v1/widget_types.go": "package v1\ntype Widget struct { APIVersion string; Kind string }\n",
			}
			for name, body := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			srv := &mcpServer{workspaceRoot: root}
			got, err := srv.toolInspectProject(map[string]interface{}{"project_path": "."})
			if err != nil {
				t.Fatal(err)
			}
			names := got["resources"].([]string)
			if len(names) != 1 || names[0] != "Widget" {
				t.Fatalf("resources = %v, want [Widget]", names)
			}
			endpoints := got["endpoints"].(map[string][]string)
			if len(endpoints) != 1 || len(endpoints["Widget"]) != 8 || endpoints["Widget"][0] != "GET /widgets" {
				t.Fatalf("endpoints = %v, want default endpoints for discovered Widget only", endpoints)
			}
		})
	}
}

func TestResourceEndpoints_UsesAPIGroupResourcePaths(t *testing.T) {
	group := &configpkg.APIGroup{
		Resources: []string{"Stale"},
		ResourcePaths: map[string]string{
			"Widget": "/inventory/widgets",
		},
	}

	got := resourceEndpoints([]string{"Widget", "Gadget"}, group)
	if len(got) != 2 {
		t.Fatalf("endpoints = %v, want only the two discovered resources", got)
	}

	wantWidget := []string{
		"GET /inventory/widgets",
		"POST /inventory/widgets",
		"GET /inventory/widgets/{uid}",
		"PUT /inventory/widgets/{uid}",
		"PATCH /inventory/widgets/{uid}",
		"DELETE /inventory/widgets/{uid}",
		"PUT /inventory/widgets/{uid}/status",
		"PATCH /inventory/widgets/{uid}/status",
	}
	assertEndpoints(t, got["Widget"], wantWidget)

	wantGadget := []string{
		"GET /gadgets",
		"POST /gadgets",
		"GET /gadgets/{uid}",
		"PUT /gadgets/{uid}",
		"PATCH /gadgets/{uid}",
		"DELETE /gadgets/{uid}",
		"PUT /gadgets/{uid}/status",
		"PATCH /gadgets/{uid}/status",
	}
	assertEndpoints(t, got["Gadget"], wantGadget)
}

func TestResourceEndpoints_ReturnsEmptyMapForNoResources(t *testing.T) {
	tests := []struct {
		name  string
		group *configpkg.APIGroup
	}{
		{name: "nil group"},
		{name: "empty group", group: &configpkg.APIGroup{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resourceEndpoints(nil, test.group)
			if len(got) != 0 {
				t.Fatalf("resourceEndpoints() = %v, want empty map", got)
			}
		})
	}
}

func TestResourceEndpoints_UsesDefaultPathWithoutGroup(t *testing.T) {
	got := resourceEndpoints([]string{"Widget"}, nil)
	if len(got["Widget"]) != 8 || got["Widget"][0] != "GET /widgets" {
		t.Fatalf("endpoints = %v, want default Widget endpoints", got)
	}
}

func assertEndpoints(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("endpoint count = %d, want %d; endpoints = %v", len(got), len(want), got)
	}
	for index, endpoint := range want {
		if got[index] != endpoint {
			t.Errorf("endpoint %d = %q, want %q", index, got[index], endpoint)
		}
	}
}
