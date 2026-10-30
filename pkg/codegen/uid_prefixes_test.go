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

func TestGeneratedRoutesWarnAboutIncompatiblePrefixes(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/uidtest")
	gen.Resources = []ResourceMetadata{{Name: "Node", URLPath: "/nodes"}}
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
	// The startup check must audit the whole registry, not just the prefixes
	// this file registers, so that hand-written registrations are covered too.
	for _, want := range []string{
		`"github.com/openchami/fabrica/pkg/uid"`,
		"func warnIncompatibleUIDPrefixes() {",
		"resource.GetRegisteredPrefixes()",
		"uid.ValidatePrefix(prefix)",
		"sort.Strings(kinds)",
	} {
		if !strings.Contains(string(generated), want) {
			t.Errorf("generated routes missing %q", want)
		}
	}
	register := strings.Index(string(generated), "func registerResourcePrefixes() error {")
	call := strings.Index(string(generated), "\twarnIncompatibleUIDPrefixes()")
	if register < 0 || call < register {
		t.Fatalf("warning is not reached from startup registration: %d, %d", register, call)
	}
}

func TestGeneratedRoutesOmitUIDValidationWithoutResources(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/uidtest")
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
	// format.Source parses without type-checking, so an import left behind for a
	// resourceless service would only fail later, when the service is built.
	for _, unwanted := range []string{
		`"github.com/openchami/fabrica/pkg/uid"`,
		`"github.com/openchami/fabrica/pkg/resource"`,
		`"sort"`,
		`"log"`,
		"warnIncompatibleUIDPrefixes",
	} {
		if strings.Contains(string(generated), unwanted) {
			t.Errorf("resourceless routes contain %q", unwanted)
		}
	}
}
