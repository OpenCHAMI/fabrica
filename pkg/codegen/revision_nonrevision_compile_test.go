// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRevisionSurfacesOnlyForEnabledResources_nonRevisionSampleCompiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	writeNonRevisionFixture(t, dir, root)

	gen := NewGenerator(dir, "main", "example.com/nonrevisionfixture")
	gen.Config.VersioningEnabled = true
	gen.Config.ConditionalEnabled = false
	gen.Resources = []ResourceMetadata{{
		Name: "Node", PluralName: "nodes", Package: "example.com/nonrevisionfixture/apis/v1", PackageAlias: "v1",
		TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", URLPath: "/nodes", StorageName: "Node",
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() for non-revision fixture error = %v", err)
	}
	for _, generator := range []struct {
		name string
		run  func() error
	}{
		{name: "models", run: gen.GenerateModels},
		{name: "handlers", run: gen.GenerateHandlers},
		{name: "routes", run: gen.GenerateRoutes},
		{name: "openapi", run: gen.GenerateOpenAPI},
	} {
		if err := generator.run(); err != nil {
			t.Fatalf("generate non-revision %s: %v", generator.name, err)
		}
	}

	clientGen := NewGenerator(filepath.Join(dir, "pkg", "client"), "client", "example.com/nonrevisionfixture")
	clientGen.Config.VersioningEnabled = true
	clientGen.Resources = gen.Resources
	if err := os.MkdirAll(clientGen.OutputDir, 0o755); err != nil {
		t.Fatalf("create non-revision client directory: %v", err)
	}
	if err := clientGen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates() for non-revision client error = %v", err)
	}
	if err := clientGen.GenerateClientModels(); err != nil {
		t.Fatalf("generate non-revision client models: %v", err)
	}
	if err := clientGen.GenerateClient(); err != nil {
		t.Fatalf("generate non-revision client: %v", err)
	}

	var storage bytes.Buffer
	if err := gen.Templates["storage"].Execute(&storage, gen.globalTemplateData("storage/file.go.tmpl")); err != nil {
		t.Fatalf("render non-revision storage: %v", err)
	}
	formattedStorage, err := format.Source(storage.Bytes())
	if err != nil {
		t.Fatalf("format non-revision storage: %v", err)
	}
	storageDir := filepath.Join(dir, "internal", "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatalf("create non-revision storage directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "storage_generated.go"), formattedStorage, 0o644); err != nil {
		t.Fatalf("write non-revision storage: %v", err)
	}

	for _, name := range []string{
		"node_handlers_generated.go", "routes_generated.go", "models_generated.go", "openapi_generated.go",
		filepath.Join("pkg", "client", "client_generated.go"), filepath.Join("pkg", "client", "models_generated.go"),
		filepath.Join("internal", "storage", "storage_generated.go"),
	} {
		generated, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read non-revision generated file %s: %v", name, err)
		}
		for _, marker := range []string{"EnsureNodeRevision", "ResolveNodeRevision", "PromoteNodeDefault", "/nodes/{name}/revisions", "/nodes/%s/revisions", "revision.NewFileStore"} {
			if strings.Contains(string(generated), marker) {
				t.Errorf("non-revision generated file %s exposes %q", name, marker)
			}
		}
	}

	for _, args := range [][]string{{"mod", "tidy"}, {"test", "-count=1", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.6")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("execute non-revision generated sample go %v: %v\n%s", args, err, output)
		}
	}
}

func writeNonRevisionFixture(t *testing.T, dir, root string) {
	t.Helper()

	files := map[string]string{
		"go.mod": fmt.Sprintf("module example.com/nonrevisionfixture\n\ngo 1.26.6\n\nrequire (\n github.com/go-chi/chi/v5 v5.2.3\n github.com/openchami/fabrica v0.0.0\n)\n\nreplace github.com/openchami/fabrica => %s\n", root),
		"apis/v1/types.go": `package v1
import "github.com/openchami/fabrica/pkg/resource"
type NodeSpec struct { Value string ` + "`json:\"value\"`" + ` }
type NodeStatus struct { Phase string ` + "`json:\"phase,omitempty\"`" + ` }
type Node struct { APIVersion string ` + "`json:\"apiVersion\"`" + `; Kind string ` + "`json:\"kind\"`" + `; Metadata resource.Metadata ` + "`json:\"metadata\"`" + `; Spec NodeSpec ` + "`json:\"spec\"`" + `; Status NodeStatus ` + "`json:\"status\"`" + ` }
`,
	}
	for relative, content := range files {
		path := filepath.Join(dir, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create non-revision fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write non-revision fixture %s: %v", relative, err)
		}
	}
}
