// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedProjectGate_detects_compile_and_routing_mutations(t *testing.T) {
	// Given
	project := newGeneratedProject(t, "ent")
	project.writeResourceSource(t, generatedAllTypesNoHashSource)
	expected := generatedProjectExpectations{dedicatedResources: []string{"Token"}}
	project.requireGeneratedProjectGate(t, expected)

	tests := []struct {
		name        string
		path        string
		old         string
		replacement string
		appendCode  string
		verify      func() error
	}{
		{
			name:        "missing generated import fails build",
			path:        filepath.Join("internal", "storage", "ent_adapter_token.go"),
			old:         "\t\"encoding/json\"\n",
			replacement: "",
			verify: func() error {
				result := project.build(t)
				if result.err == nil {
					return nil
				}
				return fmt.Errorf("%s", result.failureMessage())
			},
		},
		{
			name:        "wrong optional accessor fails build",
			path:        filepath.Join("internal", "storage", "ent_adapter_token.go"),
			old:         "SetNillableSpecDescription",
			replacement: "SetNillableSpecDescriptions",
			verify: func() error {
				result := project.build(t)
				if result.err == nil {
					return nil
				}
				return fmt.Errorf("%s", result.failureMessage())
			},
		},
		{
			name:        "dead dedicated routing fails artifact contract",
			path:        filepath.Join("internal", "storage", "storage_ent_resources_generated.go"),
			old:         "ToEntToken(",
			replacement: "ToEntResource(",
			appendCode:  "\nfunc deadTokenRouting() { _, _ = ToEntToken(nil, nil, nil) }\n",
			verify: func() error {
				return verifyGeneratedProjectArtifacts(project.root, expected)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(project.root, test.path)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read mutation target %s: %v", test.path, err)
			}
			mutated := strings.Replace(string(original), test.old, test.replacement, 1)
			mutated += test.appendCode
			if mutated == string(original) {
				t.Fatalf("mutation target %q is absent from %s", test.old, test.path)
			}
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatalf("write mutation target %s: %v", test.path, err)
			}
			t.Cleanup(func() {
				if err := os.WriteFile(path, original, 0o644); err != nil {
					t.Errorf("restore mutation target %s: %v", test.path, err)
				}
			})

			// When
			err = test.verify()

			// Then
			if err == nil {
				t.Fatalf("generated-project gate accepted mutation %q", test.name)
			}
			t.Logf("retained mutation failure [%s]: %v", test.name, err)
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatalf("restore mutation target %s: %v", test.path, err)
			}
		})
	}
}
