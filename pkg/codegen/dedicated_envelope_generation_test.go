// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"
)

func TestDedicatedEnvelope_schema_renders_complete_envelope(t *testing.T) {
	// Given
	schema := generateDedicatedShapeSchema(t, "sqlite").content

	// When
	wantFragments := []string{
		`field.JSON("status", json.RawMessage{})`,
		`field.JSON("labels", map[string]string{})`,
		`field.JSON("annotations", map[string]string{})`,
		`field.String("resource_version")`,
		`field.Time("created_at")`,
		`field.Time("updated_at")`,
	}

	// Then
	for _, fragment := range wantFragments {
		if !strings.Contains(schema, fragment) {
			t.Errorf("dedicated schema missing envelope field %q\n%s", fragment, schema)
		}
	}
}
