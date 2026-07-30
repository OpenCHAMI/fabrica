// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"reflect"
	"strings"
	"testing"
)

func TestGeneratedAnnotationProject_ent_versioning_preflight_preserves_managed_output(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "unannotated generic", source: generatedUnannotatedTokenSource},
		{name: "dedicated", source: validAnnotatedTokenSource},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := newGeneratedProject(t, "ent")
			project.writeResourceSource(t, generatedUnannotatedTokenSource)
			if result := project.generate(t); result.err != nil {
				t.Fatalf("prepare baseline: %s", result.failureMessage())
			}
			project.writeResourceSource(t, versionedGeneratedResourceSource(test.source))
			before := snapshotGeneratedManagedTree(t, project.root)

			result := project.generate(t)

			if result.err == nil {
				t.Fatal("generation succeeded for unsupported Ent resource version snapshots")
			}
			for _, expected := range []string{"Token", "+fabrica:resource-versioning=enabled", "ent"} {
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

func versionedGeneratedResourceSource(source string) string {
	return "// +fabrica:resource-versioning=enabled\n" + source
}
