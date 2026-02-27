package main

import (
	"os"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen"
)

func mustReadFile(t *testing.T, filename string) string {
	t.Helper()
	b, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", filename, err)
	}
	return string(b)
}

func mustReadTemplate(t *testing.T, name string) string {
	t.Helper()

	gen := codegen.NewGenerator("example.com/test", "cmd/server", "main")
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}

	// codegen.Generator.LoadTemplates maps symbolic names (e.g., "routes") to templates.
	// For template-level assertions we key off those names here.
	key := name
	if name == "server/routes.go.tmpl" {
		key = "routes"
	}

	tmpl, ok := gen.Templates[key]
	if !ok {
		// Some templates are stored under their full embed path.
		if key == "init/main.go.tmpl" {
			for _, alt := range []string{"init/main.go.tmpl", "init/main.go", "main", "main.go", "main.go.tmpl"} {
				if tmpl2, ok2 := gen.Templates[alt]; ok2 {
					tmpl = tmpl2
					ok = true
					break
				}
			}
		}
		if !ok {
			t.Fatalf("template %q not found in generator templates map", key)
		}
	}

	return tmpl.Tree.Root.String()
}
