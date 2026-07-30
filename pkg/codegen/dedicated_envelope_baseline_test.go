// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"strings"
	"testing"
)

func TestDedicatedEnvelope_generic_adapter_baseline_preserves_status_and_metadata_maps(t *testing.T) {
	// Given
	templateSource, err := os.ReadFile("templates/storage/adapter.go.tmpl")
	if err != nil {
		t.Fatalf("read generic adapter template: %v", err)
	}

	// When
	adapter := string(templateSource)

	// Then
	wantFragments := []string{
		"status, err = json.Marshal(v.Status)",
		`fmt.Errorf("failed to marshal status: %w", err)`,
		"json.Unmarshal(entResource.Status, &resource.Status)",
		`fmt.Errorf("failed to unmarshal status for {{.Name}}: %w", err)`,
		"Labels:      make(map[string]string)",
		"Annotations: make(map[string]string)",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(adapter, fragment) {
			t.Errorf("generic adapter baseline missing %q", fragment)
		}
	}
}
