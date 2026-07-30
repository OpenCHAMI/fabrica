// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"testing"
)

func TestGoLiteral_renders_safe_parseable_scalar_defaults(t *testing.T) {
	tests := []struct {
		name   string
		value  DefaultValue
		goType string
		want   string
	}{
		{name: "quoted string", value: StringDefault{Value: `say "hello" \\ world`}, goType: "string", want: `"say \"hello\" \\\\ world"`},
		{name: "unicode string", value: StringDefault{Value: "世界"}, goType: "string", want: `"世界"`},
		{name: "newline string", value: StringDefault{Value: "first\nsecond"}, goType: "string", want: `"first\nsecond"`},
		{name: "bool false", value: BoolDefault{Value: false}, goType: "bool", want: "false"},
		{name: "int zero", value: IntDefault{Value: 0}, goType: "int", want: "0"},
		{name: "int64", value: Int64Default{Value: 9223372036854775807}, goType: "int64", want: "9223372036854775807"},
		{name: "float64 zero", value: Float64Default{Value: 0}, goType: "float64", want: "0.0"},
		{name: "float64", value: Float64Default{Value: 1.25}, goType: "float64", want: "1.25"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := GoLiteral(tt.value)
			// Then
			if err != nil {
				t.Fatalf("GoLiteral() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GoLiteral() = %q, want %q", got, tt.want)
			}
			if _, err := parser.ParseExpr(got); err != nil {
				t.Errorf("parser.ParseExpr(%q): %v", got, err)
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "literal.go", "package literal\nvar value "+tt.goType+" = "+got+"\n", 0)
			if err != nil {
				t.Fatalf("parser.ParseFile(%q): %v", got, err)
			}
			if _, err := new(types.Config).Check("literal", fset, []*ast.File{file}, nil); err != nil {
				t.Errorf("types.Check(%q): %v", got, err)
			}
			if stringValue, ok := tt.value.(StringDefault); ok {
				unquoted, err := strconv.Unquote(got)
				if err != nil || unquoted != stringValue.Value {
					t.Errorf("quoted literal round trip = %q, %v", unquoted, err)
				}
			}
		})
	}
}
