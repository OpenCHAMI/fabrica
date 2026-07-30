// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"strings"
	"testing"
)

func TestDocumentation_contract_rejects_structural_mutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(documentationContract) documentationContract
	}{
		{
			name: "claim moved to irrelevant prose",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["README.md"]
				section := document.sections["Annotation-driven dedicated Ent storage"]
				section = strings.Replace(section, "HTTP 409", "conflict response", 1)
				document.sections["Annotation-driven dedicated Ent storage"] = section
				document.sections["Installation"] += "\nHTTP 409\n"
				contract.documents["README.md"] = document
				return contract
			},
		},
		{
			name: "evidence test renamed",
			mutate: func(contract documentationContract) documentationContract {
				delete(contract.testSymbols, "TestGeneratedSQLite_acceptance")
				contract.testSymbols["TestGeneratedSQLite_acceptance_renamed"] = struct{}{}
				return contract
			},
		},
		{
			name: "support status altered",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				section := document.sections["Tested capability matrix"]
				section = strings.Replace(section, "| Portable B-tree indexes | Supported | Supported |", "| Portable B-tree indexes | Supported | Rejected |", 1)
				document.sections["Tested capability matrix"] = section
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "field directive backend boundary removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Field directives require dedicated Ent storage", "Field directives")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "bcrypt create update distinction removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Required bcrypt create and omitted update", "Bcrypt lifecycle")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "sensitive zero presence semantics removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Sensitive zero-value update semantics", "Sensitive updates")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "persisted redacted response contract removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Persisted redacted write responses", "Write responses")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "backend common conflict contract removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Backend-common typed conflict contract", "Conflict contract")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "commit aware cursor contract removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Commit-aware migration continuation cursor", "Migration continuation cursor")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "file version snapshot support row removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "File-backed resource version snapshots", "Resource snapshots")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "ent version snapshot rejection row removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Ent resource version snapshots", "Database resource snapshots")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "strict unknown directive release note removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["CHANGELOG.md"]
				document.content = strings.ReplaceAll(document.content, "Unknown Fabrica directives now fail strictly with source-located typed parse errors", "Annotation parsing changed")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["CHANGELOG.md"] = document
				return contract
			},
		},
		{
			name: "unknown directive rejection row removed",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["pkg/annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "Unknown Fabrica directives", "Unrecognized annotations")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["pkg/annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "create defaults present and absent claims coexist",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["examples/12-storage-annotations/README.md"]
				document.content += "\nDefaults are absent from the dedicated create response.\n"
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["examples/12-storage-annotations/README.md"] = document
				return contract
			},
		},
		{
			name: "obsolete create defaults wording returns",
			mutate: func(contract documentationContract) documentationContract {
				document := contract.documents["examples/12-storage-annotations/README.md"]
				document.content = strings.ReplaceAll(document.content, "defaults are present in the dedicated create response after persisted reload", "defaults are visible after reading persisted data, not in the create handler's immediate echo")
				document.sections = parseMarkdownDocument(document.content).sections
				contract.documents["examples/12-storage-annotations/README.md"] = document
				return contract
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			contract := tt.mutate(loadDocumentationContract(t))

			// When
			errs := contract.validate()

			// Then
			if len(errs) == 0 {
				t.Fatal("mutated documentation contract passed validation")
			}
		})
	}
}
