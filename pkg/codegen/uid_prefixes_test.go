// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTypeIDPrefixes(t *testing.T) {
	for _, name := range []string{"Node", "RackTemplate", "BMC", "Group_Name", strings.Repeat("A", 63)} {
		gen := NewGenerator(t.TempDir(), "main", "example.com/uidtest")
		gen.Resources = []ResourceMetadata{{Name: name, URLPath: "/resources"}}
		if err := gen.ValidateTypeIDPrefixes(); err != nil {
			t.Fatalf("ValidateTypeIDPrefixes(%q): %v", name, err)
		}
		if err := gen.LoadTemplates(); err != nil {
			t.Fatal(err)
		}
		if err := gen.GenerateRoutes(); err != nil {
			t.Fatal(err)
		}
		generated, err := os.ReadFile(filepath.Join(gen.OutputDir, "routes_generated.go"))
		if err != nil {
			t.Fatal(err)
		}
		want := `resource.RegisterResourcePrefix("` + name + `", "` + strings.ToLower(name) + `")`
		if !strings.Contains(string(generated), want) {
			t.Fatalf("routes do not register the audited prefix: want %s", want)
		}
	}
	gen := NewGenerator(t.TempDir(), "main", "example.com/uidtest")
	if err := gen.ValidateTypeIDPrefixes(); err != nil {
		t.Fatalf("empty generator: %v", err)
	}
	for _, name := range []string{"Node1", "_Node", "Node_", "Nöde", strings.Repeat("A", 64), ""} {
		gen.Resources = append(gen.Resources, ResourceMetadata{Name: name})
	}
	err := gen.ValidateTypeIDPrefixes()
	if err == nil {
		t.Fatal("accepted incompatible prefixes")
	}
	for _, name := range []string{"Node1", "_Node", "Node_", "Nöde", strings.Repeat("A", 64), "must not be empty"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("missing diagnostic for %q in %v", name, err)
		}
	}
}

func TestTypeIDPrefixPreflightRejectsGeneration(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/uidtest")
	gen.Resources = []ResourceMetadata{{Name: "Node1", URLPath: "/nodes"}}
	if err := gen.ValidateTypeIDPrefixes(); err == nil {
		t.Fatal("preflight should flag numeric prefix")
	}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateRoutes(); err == nil {
		t.Fatal("route generation accepted numeric prefix")
	}
	if err := gen.GenerateAll(); err == nil {
		t.Fatal("GenerateAll accepted numeric prefix")
	}
	entries, err := os.ReadDir(gen.OutputDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed preflight wrote output: %v, %v", entries, err)
	}
}
