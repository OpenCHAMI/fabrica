// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package versioning

import "testing"

func TestDefaultResourceMapperPreservesPluralTokenForRegistryResolution(t *testing.T) {
	mapper := &DefaultResourceMapper{}
	got := mapper.MapResourceToKind("clusterdefaults")
	if got != "Clusterdefaults" {
		t.Fatalf("MapResourceToKind(clusterdefaults) = %q, want %q", got, "Clusterdefaults")
	}

	registry := NewVersionRegistry()
	if err := registry.RegisterVersion("ClusterDefaults", "v1", ResourceTypeInfo{Metadata: SchemaVersion{Version: "v1", IsDefault: true}}); err != nil {
		t.Fatalf("RegisterVersion: %v", err)
	}

	resolved, ok := registry.ResolveKind(got)
	if !ok || resolved != "ClusterDefaults" {
		t.Fatalf("ResolveKind(%q) = (%q, %v), want (ClusterDefaults, true)", got, resolved, ok)
	}
}
