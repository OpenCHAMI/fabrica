// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

func TestAPIVersionTemplatesIncludeGeneratedSPDXHeader(t *testing.T) {
	for _, templatePath := range []string{
		"pkg/codegen/templates/apiversion/register.gotmpl",
		"pkg/codegen/templates/apiversion/types_hub.gotmpl",
		"pkg/codegen/templates/apiversion/types_spoke.gotmpl",
	} {
		content := mustReadFile(t, templatePath)
		if !strings.Contains(content, "Copyright © {{.CopyrightYear}} OpenCHAMI a Series of LF Projects, LLC") {
			t.Fatalf("template %s missing dynamic copyright header", templatePath)
		}
		// If the line below includes a full header, it will flag the reuse test as a malformed header and fail.
		if !strings.Contains(content, "PDX-License-Identifier: MIT") {
			t.Fatalf("template %s missing SPDX license header", templatePath)
		}
	}
}

func TestAPIVersionRegisterTemplateRendersSPDXHeader(t *testing.T) {
	tmplText := mustReadFile(t, "pkg/codegen/templates/apiversion/register.gotmpl")
	tmpl, err := template.New("register").Parse(tmplText)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	data := map[string]interface{}{
		"Version":       "dev",
		"Template":      "apiversion/register.gotmpl",
		"GeneratedAt":   "2026-04-25T00:00:00Z",
		"CopyrightYear": "2025",
		"Groups": []map[string]interface{}{
			{
				"Name":           "boot.openchami.io",
				"StorageVersion": "v1",
				"Spokes":         []string{"v1"},
				"Resources":      []string{"Node"},
			},
		},
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	generated := out.String()
	if !strings.Contains(generated, "PDX-FileCopyrightText") {
		t.Fatalf("generated register template missing SPDX copyright header:\n%s", generated)
	}
	if !strings.Contains(generated, "PDX-License-Identifier: MIT") {
		t.Fatalf("generated register template missing SPDX license header:\n%s", generated)
	}
}
