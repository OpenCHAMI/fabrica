// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"
)

func TestGeneratedDedicatedSchema_default_modifiers_match_pointer_shape(t *testing.T) {
	schema := generateDedicatedShapeSchema(t, "sqlite").content

	scalar := generatedFieldBlock(t, schema, `field.Bool("spec_enabled")`)
	if strings.Contains(scalar, "Optional().") || strings.Contains(scalar, "Nillable().") {
		t.Fatalf("non-pointer scalar default is nullable:\n%s", scalar)
	}
	if !strings.Contains(scalar, "Default(false).") {
		t.Fatalf("non-pointer scalar default is absent:\n%s", scalar)
	}

	pointer := generatedFieldBlock(t, schema, `field.String("spec_description")`)
	for _, modifier := range []string{"Optional().", "Nillable().", `Default("fallback").`} {
		if !strings.Contains(pointer, modifier) {
			t.Errorf("pointer default field missing %q:\n%s", modifier, pointer)
		}
	}
}

func generatedFieldBlock(t *testing.T, schema, field string) string {
	t.Helper()
	start := strings.Index(schema, field)
	if start < 0 {
		t.Fatalf("generated schema missing %s", field)
	}
	remainder := schema[start:]
	end := strings.Index(remainder, `Comment(`)
	if end < 0 {
		t.Fatalf("generated field %s has no comment terminator", field)
	}
	return remainder[:end]
}
