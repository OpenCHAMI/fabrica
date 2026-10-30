// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedStartupWarnsAboutIncompatiblePrefixes builds the generated
// registration path and proves the startup audit reports a hand-written
// registration the generator never sees, without refusing to start.
func TestGeneratedStartupWarnsAboutIncompatiblePrefixes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated prefix warning integration test in short mode")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeRevisionFixture(t, dir, root)
	// The shared fixture ships a revision workflow test that needs handlers and
	// storage; this test compiles only the prefix registration path.
	if err := os.Remove(filepath.Join(dir, "revision_workflow_test.go")); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(dir, "main", "example.com/revisionfixture")
	gen.Resources = []ResourceMetadata{{
		Name: "Node", PluralName: "Nodes", URLPath: "/nodes",
		Package: "example.com/revisionfixture/apis/v1", PackageAlias: "v1",
		TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus",
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateRoutes(); err != nil {
		t.Fatal(err)
	}
	// routes_generated.go uses the service status vocabulary from models.
	if err := gen.GenerateModels(); err != nil {
		t.Fatal(err)
	}

	// Keep the generated imports and registration path; drop the route table,
	// which references handlers this test does not generate.
	routesPath := filepath.Join(dir, "routes_generated.go")
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, routesPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	source.WriteString("package main\n")
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "RegisterGeneratedRoutes" {
			continue
		}
		if err := format.Node(&source, fset, decl); err != nil {
			t.Fatal(err)
		}
		source.WriteString("\n")
	}
	if err := os.WriteFile(routesPath, []byte(source.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	// A registration the routes template cannot see, using a prefix that is a
	// valid legacy prefix but cannot produce a TypeID.
	legacy := `package main

import "github.com/openchami/fabrica/pkg/resource"

func init() { resource.RegisterResourcePrefix("LegacyNode", "node2") }
`
	if err := os.WriteFile(filepath.Join(dir, "legacy_prefix.go"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	check := `package main

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/resource"
)

func TestStartupPrefixAudit(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	log.SetFlags(0)
	defer log.SetOutput(nil)

	if err := registerResourcePrefixes(); err != nil {
		t.Fatalf("startup registration must not fail on a legacy-only prefix: %v", err)
	}
	out := logged.String()
	for _, want := range []string{"LegacyNode", "node2", "cannot generate new TypeID UIDs"} {
		if !strings.Contains(out, want) {
			t.Errorf("startup log missing %q, got: %s", want, out)
		}
	}
	if strings.Count(out, "WARNING") != 1 {
		t.Errorf("expected exactly one warning, got: %s", out)
	}
	if strings.Contains(out, "prefix \"node\"") {
		t.Errorf("compatible prefix was reported: %s", out)
	}

	// The legacy prefix still resolves stored identifiers to their kind.
	if kind, err := resource.GetResourceTypeFromUID("node2-1a2b3c4d"); err != nil || kind != "LegacyNode" {
		t.Errorf("legacy lookup = %q, %v", kind, err)
	}
	// ...but cannot mint new ones, which is what the warning is about.
	if value, err := resource.GenerateUIDForResource("LegacyNode"); err == nil {
		t.Errorf("legacy prefix generated %q", value)
	}
	// The compatible kind is unaffected.
	value, err := resource.GenerateUIDForResource("Node")
	if err != nil || !strings.HasPrefix(value, "node_") {
		t.Fatalf("Node UID = %q, %v", value, err)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "prefix_audit_test.go"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"mod", "tidy"}, {"test", "-count=1", "."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.10")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}
