// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
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
