// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestSecurity_bcrypt_costs_fail_with_parse_diagnostics(t *testing.T) {
	tests := []struct {
		name string
		cost string
	}{
		{name: "below minimum", cost: "3"},
		{name: "above maximum", cost: "32"},
		{name: "not numeric", cost: "expensive"},
		{name: "overflow", cost: "999999999999999999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			directive := "+fabrica:field:storage=hashed:bcrypt:cost=" + tt.cost
			filename := writeStrictFailureSource(t, directive, false)

			// When
			got, err := ParseFileAnnotations(filename)

			// Then
			if got != nil || !errors.Is(err, ErrAnnotationParse) {
				t.Fatalf("ParseFileAnnotations() = %#v, %v", got, err)
			}
			assertParseErrorContext(t, err, filename, "Token", "Value", directive, "")
		})
	}
}

func TestSecurity_defaults_fail_with_typed_source_diagnostics(t *testing.T) {
	tests := []struct {
		name       string
		goType     string
		directives []string
		kind       DefaultErrorKind
	}{
		{name: "transformed conflict", goType: "string", directives: []string{"+fabrica:field:storage=hashed:bcrypt", "+fabrica:field:default=secret"}, kind: DefaultErrorConflict},
		{name: "immutable conflict", goType: "string", directives: []string{"+fabrica:field:immutable", "+fabrica:field:default=pending"}, kind: DefaultErrorConflict},
		{name: "invalid bool", goType: "bool", directives: []string{"+fabrica:field:default=truthy"}, kind: DefaultErrorInvalidLiteral},
		{name: "overflow int", goType: "int", directives: []string{"+fabrica:field:default=" + strconv.FormatUint(math.MaxUint64, 10)}, kind: DefaultErrorInvalidLiteral},
		{name: "nonfinite float", goType: "float64", directives: []string{"+fabrica:field:default=NaN"}, kind: DefaultErrorInvalidLiteral},
		{name: "slice default", goType: "[]string", directives: []string{"+fabrica:field:default=value"}, kind: DefaultErrorUnsupportedType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeDefaultSource(t, tt.goType, tt.directives...)

			// When
			got, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)

			// Then
			if got != nil || !errors.Is(err, ErrInvalidDefault) {
				t.Fatalf("ResolveStorageIntent() = %#v, %v", got, err)
			}
			var defaultErr *DefaultError
			if !errors.As(err, &defaultErr) {
				t.Fatalf("errors.As(*DefaultError) = false: %v", err)
			}
			if defaultErr.Kind != tt.kind || defaultErr.Filename != filename || defaultErr.Line <= 0 || defaultErr.Column <= 0 || defaultErr.TypeName != "RecordSpec" || defaultErr.FieldName != "Value" || defaultErr.Directive == "" {
				t.Fatalf("DefaultError context = %#v", defaultErr)
			}
		})
	}
}

func TestSecurity_sensitive_standard_field_remains_explicit_in_resolved_intent(t *testing.T) {
	// Given
	filename := writeCapabilitySource(t, "string", "// +fabrica:field:sensitive\n")

	// When
	got, err := ResolveStorageIntent(filename, "Record", DialectSQLite)

	// Then
	if err != nil {
		t.Fatalf("ResolveStorageIntent() error = %v", err)
	}
	field := got.Fields[0]
	if !field.Sensitive || field.Transform.Kind != TransformStandard {
		t.Fatalf("standard sensitive intent = %#v", field)
	}
}

func TestSecurity_declaration_orders_and_cache_paths_are_equivalent(t *testing.T) {
	// Given
	resetGlobalCache(t)
	resourceFirst := writeAnnotationFixture(t, declarationOrderSource(false))
	specFirst := writeAnnotationFixture(t, declarationOrderSource(true))

	// When
	want, err := ResolveStorageIntent(resourceFirst, "Record", DialectPostgreSQL)
	if err != nil {
		t.Fatalf("resource-first ResolveStorageIntent() error = %v", err)
	}
	cold, err := ResolveStorageIntent(specFirst, "Record", DialectPostgreSQL)
	if err != nil {
		t.Fatalf("spec-first cold ResolveStorageIntent() error = %v", err)
	}
	warm, err := ResolveStorageIntent(specFirst, "Record", DialectPostgreSQL)

	// Then
	if err != nil {
		t.Fatalf("spec-first warm ResolveStorageIntent() error = %v", err)
	}
	want.Source.Filename, cold.Source.Filename, warm.Source.Filename = "", "", ""
	want.Source.Line, cold.Source.Line, warm.Source.Line = 0, 0, 0
	want.Source.Column, cold.Source.Column, warm.Source.Column = 0, 0, 0
	for index := range want.Fields {
		want.Fields[index].Source.Filename = ""
		cold.Fields[index].Source.Filename = ""
		warm.Fields[index].Source.Filename = ""
		want.Fields[index].Source.Line, cold.Fields[index].Source.Line, warm.Fields[index].Source.Line = 0, 0, 0
		want.Fields[index].Source.Column, cold.Fields[index].Source.Column, warm.Fields[index].Source.Column = 0, 0, 0
	}
	if !reflect.DeepEqual(want, cold) || !reflect.DeepEqual(cold, warm) {
		t.Fatalf("order/cache mismatch\nresource-first: %#v\nspec-first cold: %#v\nspec-first warm: %#v", want, cold, warm)
	}
}

func declarationOrderSource(specFirst bool) string {
	resource := "// +fabrica:resource\n// +fabrica:storage=dedicated\ntype Record struct { Spec RecordSpec }\n"
	spec := "type RecordSpec struct {\n\t// +fabrica:field:sensitive\n\tValue string\n}\n"
	if specFirst {
		return "package test\n\n" + spec + "\n" + resource
	}
	return "package test\n\n" + resource + "\n" + spec
}
