// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	configpkg "github.com/openchami/fabrica/internal/config"
)

func TestResourceEndpoints_UsesAPIGroupResourcePaths(t *testing.T) {
	group := &configpkg.APIGroup{
		Resources: []string{"Widget", "Gadget"},
		ResourcePaths: map[string]string{
			"Widget": "/inventory/widgets",
		},
	}

	got := resourceEndpoints(group)

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

func TestResourceEndpoints_ReturnsEmptyMapForNilOrEmptyGroup(t *testing.T) {
	tests := []struct {
		name  string
		group *configpkg.APIGroup
	}{
		{name: "nil group"},
		{name: "empty group", group: &configpkg.APIGroup{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := resourceEndpoints(test.group)
			if len(got) != 0 {
				t.Fatalf("resourceEndpoints() = %v, want empty map", got)
			}
		})
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
