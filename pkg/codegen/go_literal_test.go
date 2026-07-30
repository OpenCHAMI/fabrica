// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"go/parser"
	"testing"
	"text/template"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestGoLiteral_template_helper_is_registered_with_typed_contract(t *testing.T) {
	// Given
	tmpl, err := template.New("literal").Funcs(templateFuncs).Parse(`{{ goLiteral . }}`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	// When
	var output bytes.Buffer
	err = tmpl.Execute(&output, annotations.StringDefault{Value: "\"; panic(unsafe) //\nnext"})
	// Then
	if err != nil {
		t.Fatalf("execute template: %v", err)
	}
	if _, err := parser.ParseExpr(output.String()); err != nil {
		t.Fatalf("rendered helper output %q is not a Go expression: %v", output.String(), err)
	}
}
