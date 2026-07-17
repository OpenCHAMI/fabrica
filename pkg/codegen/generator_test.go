// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"
)

func TestTemplateDataIncludesCopyrightYear(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.Version = "test"

	resource := ResourceMetadata{
		Name:         "Node",
		PluralName:   "nodes",
		Package:      "example.com/test/pkg/resources/node",
		PackageAlias: "node",
		TypeName:     "*node.Node",
		SpecType:     "node.NodeSpec",
		StatusType:   "node.NodeStatus",
		URLPath:      "/nodes",
		StorageName:  "Node",
	}

	data := gen.templateData(resource, "server/handlers.go.tmpl")
	if got, ok := data["CopyrightYear"].(int); !ok || got != time.Now().UTC().Year() {
		t.Fatalf("templateData CopyrightYear = %v, want %d", data["CopyrightYear"], time.Now().UTC().Year())
	}
}

func TestGlobalAndMiddlewareTemplateDataIncludeCopyrightYear(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.Version = "test"

	for name, data := range map[string]map[string]interface{}{
		"global":     gen.globalTemplateData("server/models.go.tmpl"),
		"middleware": gen.middlewareData("middleware/validation.go.tmpl"),
	} {
		if got, ok := data["CopyrightYear"].(int); !ok || got != time.Now().UTC().Year() {
			t.Fatalf("%s CopyrightYear = %v, want %d", name, data["CopyrightYear"], time.Now().UTC().Year())
		}
	}
}

func TestStorageDisabledHandlersUsePersistenceHooks(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.Version = "test"
	gen.Config.StorageEnabled = false
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}

	resource := ResourceMetadata{
		Name:         "Node",
		PluralName:   "nodes",
		Package:      "example.com/test/pkg/resources/node",
		PackageAlias: "node",
		TypeName:     "*node.Node",
		SpecType:     "node.NodeSpec",
		StatusType:   "node.NodeStatus",
		URLPath:      "/nodes",
		StorageName:  "Node",
		Operations: ResourceOperations{
			Get:    true,
			Create: true,
			Delete: true,
		},
	}

	data := gen.templateData(resource, "server/handlers.go.tmpl")
	var handlers bytes.Buffer
	if err := gen.Templates["handlers"].Execute(&handlers, data); err != nil {
		t.Fatalf("execute handlers template: %v", err)
	}
	gotHandlers := handlers.String()
	if strings.Contains(gotHandlers, "internal/storage") {
		t.Fatalf("storage-disabled handlers should not import generated storage:\n%s", gotHandlers)
	}
	wantHandlerSummary := `// Generated handlers provide:
//   - GET /nodes/{uid} (get specific Node)
//   - POST /nodes (create new Node)
//   - DELETE /nodes/{uid} (delete Node)`
	if !strings.Contains(gotHandlers, wantHandlerSummary) {
		t.Fatalf("generated handler summary is not compact and readable:\n%s", gotHandlers)
	}
	for _, want := range []string{"GetNodeResource(r.Context(), uid)", "SaveNodeResource(r.Context(), node)", "DeleteNodeResource(r.Context(), uid)"} {
		if !strings.Contains(gotHandlers, want) {
			t.Fatalf("handlers missing %q:\n%s", want, gotHandlers)
		}
	}

	gen.Resources = []ResourceMetadata{resource}
	var routes bytes.Buffer
	if err := gen.Templates["routes"].Execute(&routes, gen.globalTemplateData("server/routes.go.tmpl")); err != nil {
		t.Fatalf("execute routes template: %v", err)
	}
	wantRouteSummary := `// Route patterns:
//   - GET    /nodes/{uid}        -> Get specific Node
//   - POST   /nodes              -> Create new Node
//   - DELETE /nodes/{uid}        -> Delete Node`
	if !strings.Contains(routes.String(), wantRouteSummary) {
		t.Fatalf("generated route summary is not compact and readable:\n%s", routes.String())
	}
	var openAPI bytes.Buffer
	if err := gen.Templates["openapi"].Execute(&openAPI, gen.globalTemplateData("server/openapi.go.tmpl")); err != nil {
		t.Fatalf("execute OpenAPI template: %v", err)
	}

	var hooks bytes.Buffer
	if err := gen.Templates["persistenceHooks"].Execute(&hooks, data); err != nil {
		t.Fatalf("execute persistence hooks template: %v", err)
	}
	gotHooks := hooks.String()
	if strings.Contains(gotHooks, "internal/storage") {
		t.Fatalf("storage-disabled hooks should not import generated storage:\n%s", gotHooks)
	}
	for _, want := range []string{
		"GetNodeResource must be implemented when storage generation is disabled",
		"SaveNodeResource must be implemented when storage generation is disabled",
		"DeleteNodeResource must be implemented when storage generation is disabled",
	} {
		if !strings.Contains(gotHooks, want) {
			t.Fatalf("hooks missing %q:\n%s", want, gotHooks)
		}
	}
}

func TestStorageEnabledPersistenceHooksUseGeneratedStorage(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.Version = "test"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}

	resource := ResourceMetadata{
		Name:         "Node",
		PluralName:   "nodes",
		Package:      "example.com/test/pkg/resources/node",
		PackageAlias: "node",
		TypeName:     "*node.Node",
		SpecType:     "node.NodeSpec",
		StatusType:   "node.NodeStatus",
		URLPath:      "/nodes",
		StorageName:  "Node",
		Operations: ResourceOperations{
			List:   true,
			Create: true,
		},
	}

	var hooks bytes.Buffer
	if err := gen.Templates["persistenceHooks"].Execute(
		&hooks,
		gen.templateData(resource, "server/persistence_hooks.go.tmpl"),
	); err != nil {
		t.Fatalf("execute persistence hooks template: %v", err)
	}

	got := hooks.String()
	for _, want := range []string{
		`"example.com/test/internal/storage"`,
		"return storage.LoadAllNodes(ctx)",
		"return storage.SaveNode(ctx, resource)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("storage-enabled persistence hooks missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "must be implemented when storage generation is disabled") {
		t.Fatalf("storage-enabled persistence hooks contain disabled-storage stubs:\n%s", got)
	}
}

func TestPersistenceHooksOnlyWhenAbsent(t *testing.T) {
	outputDir := t.TempDir()
	gen := NewGenerator(outputDir, "main", "example.com/test")
	gen.Version = "test"
	gen.Config.StorageEnabled = false
	gen.Resources = []ResourceMetadata{
		{
			Name:         "Node",
			PluralName:   "nodes",
			Package:      "example.com/test/pkg/resources/node",
			PackageAlias: "node",
			TypeName:     "*node.Node",
			SpecType:     "node.NodeSpec",
			StatusType:   "node.NodeStatus",
			URLPath:      "/nodes",
			StorageName:  "Node",
			Operations:   ResourceOperations{List: true},
		},
	}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if err := gen.GenerateHandlers(); err != nil {
		t.Fatalf("GenerateHandlers: %v", err)
	}

	hookPath := filepath.Join(outputDir, "node_persistence_hooks.go")
	generated, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read persistence hook file: %v", err)
	}
	if !strings.Contains(string(generated), "func ListNodeResources") {
		t.Fatalf("persistence hook file missing list hook:\n%s", generated)
	}

	customized := append(generated, []byte("\n// project customization\n")...)
	if err := os.WriteFile(hookPath, customized, 0644); err != nil {
		t.Fatalf("customize persistence hook file: %v", err)
	}
	if err := gen.GenerateHandlers(); err != nil {
		t.Fatalf("regenerate handlers: %v", err)
	}

	afterRegeneration, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read persistence hook file after regeneration: %v", err)
	}
	if !bytes.Equal(afterRegeneration, customized) {
		t.Fatal("regeneration overwrote the project-owned persistence hook file")
	}
}

func TestExecuteTemplateBackfillsCommonMetadata(t *testing.T) {
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.Version = "test"
	gen.Templates = map[string]*template.Template{
		"copyright": template.Must(template.New("copyright").Parse("{{.Version}}|{{.Template}}|{{.CopyrightYear}}")),
	}

	t.Run("nil data", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "out.txt")
		if err := gen.executeTemplate("copyright", outputPath, nil); err != nil {
			t.Fatalf("executeTemplate(nil): %v", err)
		}

		gotBytes, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		want := strings.Join([]string{"test", "copyright", strconv.Itoa(time.Now().UTC().Year())}, "|")
		if got := strings.TrimSpace(string(gotBytes)); got != want {
			t.Fatalf("executeTemplate(nil) = %q, want %q", got, want)
		}
	})

	t.Run("map data", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "out.txt")
		if err := gen.executeTemplate("copyright", outputPath, map[string]interface{}{"Template": "custom.tmpl"}); err != nil {
			t.Fatalf("executeTemplate(map): %v", err)
		}

		gotBytes, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		want := strings.Join([]string{"test", "custom.tmpl", strconv.Itoa(time.Now().UTC().Year())}, "|")
		if got := strings.TrimSpace(string(gotBytes)); got != want {
			t.Fatalf("executeTemplate(map) = %q, want %q", got, want)
		}
	})
}

func TestWriteGeneratedFileSkipsTimestampOnlyChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.go")
	original := []byte("// Code generated by Fabrica v0.4.2. DO NOT EDIT.\n// Generated: 2026-05-05T00:00:00Z\npackage test\n")
	updated := []byte("// Code generated by Fabrica v0.4.3-dirty (commit: deadbeef). DO NOT EDIT.\n// Generated: 2026-05-05T00:00:01Z\npackage test\n")

	written, err := writeGeneratedFile(path, original)
	if err != nil {
		t.Fatalf("writeGeneratedFile(original): %v", err)
	}
	if !written {
		t.Fatal("expected initial write to write content")
	}

	written, err = writeGeneratedFile(path, updated)
	if err != nil {
		t.Fatalf("writeGeneratedFile(updated): %v", err)
	}
	if written {
		t.Fatal("expected timestamp-only change to be skipped")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file content changed unexpectedly: got %q want %q", got, original)
	}
}

func TestWriteGeneratedFileWritesRealChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.go")
	original := []byte("// Code generated by Fabrica v0.4.2. DO NOT EDIT.\n// Generated: 2026-05-05T00:00:00Z\npackage test\n")
	updated := []byte("// Code generated by Fabrica v0.4.3-dirty. DO NOT EDIT.\n// Generated: 2026-05-05T00:00:01Z\npackage test\n\nfunc x() {}\n")

	if _, err := writeGeneratedFile(path, original); err != nil {
		t.Fatalf("writeGeneratedFile(original): %v", err)
	}

	written, err := writeGeneratedFile(path, updated)
	if err != nil {
		t.Fatalf("writeGeneratedFile(updated): %v", err)
	}
	if !written {
		t.Fatal("expected real change to be written")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, updated) {
		t.Fatalf("file content mismatch: got %q want %q", got, updated)
	}
}
