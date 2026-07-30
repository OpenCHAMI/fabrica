// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestGeneratedAnnotationProject_backend_preflight_failure_preserves_managed_output(t *testing.T) {
	tests := []struct {
		name     string
		backend  string
		source   string
		expected []string
	}{
		{name: "generic Ent field directive", backend: "ent", source: generatedGenericTokenSourceWithDirective("+fabrica:field:sensitive"), expected: []string{"TokenSpec", "Value", "+fabrica:field:sensitive"}},
		{name: "generic file field directive", backend: "file", source: generatedGenericTokenSourceWithDirective("+fabrica:field:sensitive"), expected: []string{"TokenSpec", "Value", "+fabrica:field:sensitive"}},
		{name: "dedicated file mode", backend: "file", source: validAnnotatedTokenSource, expected: []string{"Token", "+fabrica:storage=dedicated"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := newGeneratedProject(t, test.backend)
			project.writeResourceSource(t, generatedUnannotatedTokenSource)
			if result := project.generate(t); result.err != nil {
				t.Fatalf("prepare baseline: %s", result.failureMessage())
			}
			project.writeResourceSource(t, test.source)
			before := snapshotGeneratedManagedTree(t, project.root)

			result := project.generate(t)

			if result.err == nil {
				t.Fatalf("generation succeeded for unsupported %s storage contract", test.backend)
			}
			for _, expected := range append([]string{"token_types.go"}, test.expected...) {
				if !strings.Contains(result.stdout+result.stderr, expected) {
					t.Errorf("failure missing %q\n%s", expected, result.failureMessage())
				}
			}
			after := snapshotGeneratedManagedTree(t, project.root)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("preflight failure mutated managed output\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

func TestGeneratedAnnotationProject_generic_without_field_directives_builds(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		source  string
	}{
		{name: "implicit generic Ent", backend: "ent", source: generatedUnannotatedTokenSource},
		{name: "explicit generic Ent", backend: "ent", source: generatedExplicitGenericTokenSource},
		{name: "implicit generic file", backend: "file", source: generatedUnannotatedTokenSource},
		{name: "explicit generic file", backend: "file", source: generatedExplicitGenericTokenSource},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := newGeneratedProject(t, test.backend)
			project.writeResourceSource(t, test.source)

			result := project.generate(t)
			if result.err == nil {
				result = project.tidy(t)
			}
			if result.err == nil && test.backend == "ent" {
				result = project.run(t, "generic-ent-codegen", project.root, "go", "generate", "./internal/storage")
			}
			if result.err == nil {
				result = project.build(t)
			}

			if result.err != nil {
				t.Fatalf("%s", result.failureMessage())
			}
		})
	}
}

func generatedGenericTokenSourceWithDirective(directive string) string {
	return strings.Replace(
		generatedUnannotatedTokenSource,
		"type TokenSpec struct { Value string }",
		fmt.Sprintf("type TokenSpec struct {\n\t// %s\n\tValue string\n}", directive),
		1,
	)
}

var generatedExplicitGenericTokenSource = strings.Replace(
	generatedUnannotatedTokenSource,
	"type Token struct {",
	"// +fabrica:resource\n// +fabrica:storage=generic\ntype Token struct {",
	1,
)
