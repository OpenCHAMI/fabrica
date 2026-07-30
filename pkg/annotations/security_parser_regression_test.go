// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityUnknown_directives_fail_closed_with_typed_source_diagnostics(t *testing.T) {
	tests := []struct {
		name       string
		directive  string
		resource   bool
		suggestion string
	}{
		{name: "misspelled sensitive", directive: "+fabrica:field:sensitve", suggestion: "sensitive"},
		{name: "near miss sensitive", directive: "+fabrica:field:sensitiv", suggestion: "sensitive"},
		{name: "unknown field key", directive: "+fabrica:field:secret"},
		{name: "unknown resource key", directive: "+fabrica:retension=dedicated", resource: true},
		{name: "near miss resource storage", directive: "+fabrica:storag=dedicated", resource: true, suggestion: "storage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeStrictFailureSource(t, tt.directive, tt.resource)

			// When
			got, err := ParseFileAnnotations(filename)

			// Then
			if got != nil || !errors.Is(err, ErrAnnotationParse) {
				t.Fatalf("ParseFileAnnotations() = %#v, %v", got, err)
			}
			field := "Value"
			if tt.resource {
				field = ""
			}
			assertParseErrorContext(t, err, filename, "Token", field, tt.directive, tt.suggestion)
		})
	}
}

func TestSecurityUnknown_malformed_and_duplicate_directives_fail_closed(t *testing.T) {
	tests := []struct {
		name       string
		directives []string
	}{
		{name: "empty segment", directives: []string{"+fabrica:field::sensitive"}},
		{name: "trailing segment", directives: []string{"+fabrica:field:sensitive:"}},
		{name: "missing storage value", directives: []string{"+fabrica:field:storage"}},
		{name: "duplicate sensitive", directives: []string{"+fabrica:field:sensitive", "+fabrica:field:sensitive"}},
		{name: "conflicting transforms", directives: []string{"+fabrica:field:storage=hashed:bcrypt", "+fabrica:field:storage=default"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeStrictFieldSource(t, tt.directives)

			// When
			got, err := ParseFileAnnotations(filename)

			// Then
			if got != nil || !errors.Is(err, ErrAnnotationParse) {
				t.Fatalf("ParseFileAnnotations() = %#v, %v", got, err)
			}
			assertParseErrorContext(t, err, filename, "Token", "Value", "", "")
		})
	}
}

func TestSecurityUnknown_bcrypt_parameters_reject_unknown_or_duplicate_keys(t *testing.T) {
	tests := []struct {
		name      string
		directive string
	}{
		{name: "unknown parameter", directive: "+fabrica:field:storage=hashed:bcrypt:cosst=12"},
		{name: "bare parameter", directive: "+fabrica:field:storage=hashed:bcrypt:cost"},
		{name: "duplicate cost", directive: "+fabrica:field:storage=hashed:bcrypt:cost=10:cost=12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeStrictFailureSource(t, tt.directive, false)

			// When
			got, err := ParseFileAnnotations(filename)

			// Then
			if got != nil || !errors.Is(err, ErrAnnotationParse) {
				t.Fatalf("ParseFileAnnotations() = %#v, %v", got, err)
			}
			assertParseErrorContext(t, err, filename, "Token", "Value", tt.directive, "")
		})
	}
}

func writeStrictFailureSource(t *testing.T, directive string, resource bool) string {
	t.Helper()
	if resource {
		filename := filepath.Join(t.TempDir(), "token.go")
		source := "package test\n\n// " + directive + "\ntype Token struct { Value string }\n"
		if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		return filename
	}
	return writeStrictFieldSource(t, []string{directive})
}

func writeStrictFieldSource(t *testing.T, directives []string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "token.go")
	source := "package test\n\ntype Token struct {\n"
	for _, directive := range directives {
		source += "\t// " + directive + "\n"
	}
	source += "\tValue string\n}\n"
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}
