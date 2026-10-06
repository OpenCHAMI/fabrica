// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestEntMutationAttemptLimitGeneration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured int
		want       int
	}{
		{"omitted", 0, 8}, {"single attempt", 1, 1}, {"custom", 3, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gen := NewGenerator(t.TempDir(), "main", "example.com/test")
			if gen.Config.MutationMaxAttempts != 8 {
				t.Fatalf("generator default=%d, want 8", gen.Config.MutationMaxAttempts)
			}
			gen.Config.MutationMaxAttempts = tc.configured
			gen.StorageType = "ent"
			if err := gen.LoadTemplates(); err != nil {
				t.Fatal(err)
			}
			var source bytes.Buffer
			if err := gen.Templates["storageEnt"].Execute(&source, gen.globalTemplateData("storage/ent.go.tmpl")); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				fmt.Sprintf("const entMutationMaxAttempts = %d", tc.want),
				"attempt < entMutationMaxAttempts",
				"attempt+1 >= entMutationMaxAttempts",
				"// Package storage",
			} {
				if !strings.Contains(source.String(), want) {
					t.Fatalf("rendered storage missing %q", want)
				}
			}
		})
	}
}

func TestGenerateStorageRejectsNegativeMutationLimit(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.StorageType = "ent"
	gen.Config.MutationMaxAttempts = -1
	if err := gen.GenerateStorage(); err == nil || !strings.Contains(err.Error(), "mutation_max_attempts") {
		t.Fatalf("negative limit error=%v", err)
	}
}
