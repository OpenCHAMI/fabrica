// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/openchami/fabrica/pkg/codegen"
)

func mustReadFile(t *testing.T, filename string) string {
	t.Helper()
	b, err := os.ReadFile(filename)
	if err != nil {
		// Tests in cmd/fabrica can run with different working directories depending on
		// invocation; fall back to resolving relative to repo root.
		if b2, err2 := os.ReadFile(filepath.Join("..", "..", filename)); err2 == nil {
			b = b2
			err = nil
		}
	}
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
	if name == "server/authz_classifier.go.tmpl" {
		key = "authzClassifier"
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

	return tmpl.Root.String()
}

func mustRenderInitTemplate(t *testing.T, templateText string, data templateData) string {
	t.Helper()

	tmpl, err := template.New("init").Parse(templateText)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return buf.String()
}
