// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestDefault_characterizes_currently_accepted_directives(t *testing.T) {
	tests := []struct {
		name      string
		directive string
		want      string
	}{
		{name: "string", directive: "pending", want: "pending"},
		{name: "bool", directive: "false", want: "false"},
		{name: "integer", directive: "0", want: "0"},
		{name: "float", directive: "0.0", want: "0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			source := "package test\n\ntype RecordSpec struct {\n\t// +fabrica:field:default=" + tt.directive + "\n\tValue string\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "record.go", source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Given source: %v", err)
			}
			declaration := file.Decls[0].(*ast.GenDecl)
			typeSpec := declaration.Specs[0].(*ast.TypeSpec)

			// When
			got, err := ParseResourceAnnotations(typeSpec, declaration.Doc)
			// Then
			if err != nil {
				t.Fatalf("ParseResourceAnnotations() error = %v", err)
			}
			if got.Fields["Value"].Default != tt.want {
				t.Errorf("Default = %q, want %q", got.Fields["Value"].Default, tt.want)
			}
		})
	}
}
