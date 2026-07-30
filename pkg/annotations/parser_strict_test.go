// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestParseResourceAnnotations_preserves_valid_resource_and_field_syntax(t *testing.T) {
	// Given
	const source = `package test

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	// +fabrica:field:index=gin
	// +fabrica:field:default=hidden
	// +fabrica:field:unique
	Value string
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "valid.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Given valid Go source: %v", err)
	}
	decl := file.Decls[0].(*ast.GenDecl)
	typeSpec := decl.Specs[0].(*ast.TypeSpec)

	// When
	got, err := ParseResourceAnnotations(typeSpec, decl.Doc)

	// Then
	if err != nil {
		t.Fatalf("ParseResourceAnnotations() error = %v", err)
	}
	if !got.IsResource || got.StorageMode != StorageModeDedicated {
		t.Fatalf("resource annotations = %#v", got)
	}
	field := got.Fields["Value"]
	if field == nil {
		t.Fatal("Value field annotations missing")
	}
	if !field.Sensitive || !field.Immutable || !field.Unique {
		t.Fatalf("field flags = %#v", field)
	}
	if field.Storage == nil || field.Storage.Type != StorageTypeHashed || field.Storage.Hash == nil || field.Storage.Hash.Cost != 12 {
		t.Fatalf("field storage = %#v", field.Storage)
	}
	if field.Index == nil || field.Index.Type != IndexTypeGIN || field.Default != "hidden" {
		t.Fatalf("field index/default = %#v/%q", field.Index, field.Default)
	}
}

func TestParseFileAnnotations_rejects_invalid_directives_with_source_context(t *testing.T) {
	tests := []struct {
		name       string
		directive  string
		field      string
		suggestion string
	}{
		{name: "malformed resource directive", directive: "+fabrica:resource=invalid"},
		{name: "malformed field directive", directive: "+fabrica:field:storage", field: "Value"},
		{name: "trailing field directive", directive: "+fabrica:field:sensitive:", field: "Value"},
		{name: "unknown resource key", directive: "+fabrica:storag=dedicated", suggestion: "storage"},
		{name: "unknown field key", directive: "+fabrica:field:sensitve", field: "Value", suggestion: "sensitive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := filepath.Join(t.TempDir(), "token.go")
			source := "package test\n\ntype Token struct {\n\t// " + tt.directive + "\n\tValue string\n}\n"
			if tt.field == "" {
				source = "package test\n\n// " + tt.directive + "\ntype Token struct { Value string }\n"
			}
			if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
				t.Fatalf("Given source file: %v", err)
			}

			// When
			got, err := ParseFileAnnotations(filename)

			// Then
			if err == nil {
				t.Fatalf("ParseFileAnnotations() error = nil, result = %#v", got)
			}
			if got != nil {
				t.Fatalf("ParseFileAnnotations() returned partial result %#v", got)
			}
			assertParseErrorContext(t, err, filename, "Token", tt.field, tt.directive, tt.suggestion)
		})
	}
}

func TestParseFileAnnotations_rejects_duplicate_or_conflicting_directives(t *testing.T) {
	tests := []struct {
		name       string
		directives string
		field      string
	}{
		{name: "duplicate resource directive", directives: "// +fabrica:resource\n// +fabrica:resource"},
		{name: "conflicting storage directives", directives: "// +fabrica:storage=generic\n// +fabrica:storage=dedicated"},
		{name: "duplicate field directive", directives: "// +fabrica:field:sensitive\n\t// +fabrica:field:sensitive", field: "Value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := filepath.Join(t.TempDir(), "token.go")
			source := "package test\n\n" + tt.directives + "\ntype Token struct { Value string }\n"
			if tt.field != "" {
				source = "package test\n\ntype Token struct {\n\t" + tt.directives + "\n\tValue string\n}\n"
			}
			if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
				t.Fatalf("Given source file: %v", err)
			}

			// When
			got, err := ParseFileAnnotations(filename)

			// Then
			if err == nil {
				t.Fatalf("ParseFileAnnotations() error = nil, result = %#v", got)
			}
			if got != nil {
				t.Fatalf("ParseFileAnnotations() returned partial result %#v", got)
			}
			assertParseErrorContext(t, err, filename, "Token", tt.field, "", "")
		})
	}
}

func assertParseErrorContext(
	t *testing.T,
	err error,
	filename, typeName, fieldName, directive, suggestion string,
) {
	t.Helper()

	if !errors.Is(err, ErrAnnotationParse) {
		t.Fatalf("errors.Is(ErrAnnotationParse) = false: %v", err)
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("errors.As(%T) = false: %v", parseErr, err)
	}
	if parseErr.Filename != filename || parseErr.TypeName != typeName || parseErr.FieldName != fieldName {
		t.Errorf("ParseError source = %q %q %q, want %q %q %q", parseErr.Filename, parseErr.TypeName, parseErr.FieldName, filename, typeName, fieldName)
	}
	if directive != "" && parseErr.Directive != directive {
		t.Errorf("ParseError.Directive = %q, want %q", parseErr.Directive, directive)
	}
	if parseErr.Suggestion != suggestion {
		t.Errorf("ParseError.Suggestion = %q, want %q", parseErr.Suggestion, suggestion)
	}
	if parseErr.Line <= 0 || parseErr.Column <= 0 || parseErr.Message == "" {
		t.Errorf("ParseError position/message = %d:%d %q", parseErr.Line, parseErr.Column, parseErr.Message)
	}
}
