// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
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
