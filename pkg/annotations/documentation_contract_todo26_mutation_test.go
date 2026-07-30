// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"strings"
	"testing"
)

func TestDocumentation_contract_rejects_Todo26_mutations(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		oldContract string
		replacement string
	}{
		{
			name:        "authoritative proposal notice removed",
			path:        "CHANGELOG.md",
			oldContract: "historical annotation-layer proposal is non-authoritative",
			replacement: "proposal documentation updated",
		},
		{
			name:        "request body configuration removed",
			path:        "README.md",
			oldContract: "--request-body-max-bytes",
			replacement: "request body option",
		},
		{
			name:        "request body validation removed",
			path:        "README.md",
			oldContract: "exactly one JSON value",
			replacement: "JSON input",
		},
		{
			name:        "PATCH conflict mapping removed",
			path:        "docs/guides/storage-ent.md",
			oldContract: "PATCH storage conflicts return HTTP 409",
			replacement: "PATCH errors are returned",
		},
		{
			name:        "generic metadata persistence removed",
			path:        "docs/guides/storage-ent.md",
			oldContract: "generic Ent updates persist Namespace and ResourceVersion",
			replacement: "generic Ent updates persist metadata",
		},
		{
			name:        "legacy compile compatibility removed",
			path:        "README.md",
			oldContract: "legacy embedded-resource projects compile with file and Ent storage",
			replacement: "legacy projects remain compatible",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			contract := loadDocumentationContract(t)
			document := contract.documents[test.path]
			document.content = strings.ReplaceAll(document.content, test.oldContract, test.replacement)
			document.sections = parseMarkdownDocument(document.content).sections
			contract.documents[test.path] = document

			// When
			errs := contract.validate()

			// Then
			if len(errs) == 0 {
				t.Fatal("mutated Todo 26 documentation contract passed validation")
			}
		})
	}
}
