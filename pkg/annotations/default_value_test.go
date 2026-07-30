// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestDefault_resolves_supported_scalar_values(t *testing.T) {
	tests := []struct {
		name   string
		goType string
		raw    string
		want   DefaultValue
	}{
		{name: "empty string", goType: "string", raw: "", want: StringDefault{Value: ""}},
		{name: "string", goType: "string", raw: `say"\\世界`, want: StringDefault{Value: `say"\\世界`}},
		{name: "bool true", goType: "bool", raw: "true", want: BoolDefault{Value: true}},
		{name: "bool false", goType: "bool", raw: "false", want: BoolDefault{Value: false}},
		{name: "int", goType: "int", raw: "42", want: IntDefault{Value: 42}},
		{name: "int zero", goType: "int", raw: "0", want: IntDefault{Value: 0}},
		{name: "int64", goType: "int64", raw: "9223372036854775807", want: Int64Default{Value: math.MaxInt64}},
		{name: "float64", goType: "float64", raw: "1.25", want: Float64Default{Value: 1.25}},
		{name: "float64 zero", goType: "float64", raw: "0.0", want: Float64Default{Value: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeDefaultSource(t, tt.goType, "+fabrica:field:default="+tt.raw)

			// When
			got, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)
			// Then
			if err != nil {
				t.Fatalf("ResolveStorageIntent() error = %v", err)
			}
			if got.Fields[0].Default == nil {
				t.Fatal("Default = nil")
			}
			if !reflect.DeepEqual(got.Fields[0].Default, tt.want) {
				t.Errorf("Default = %#v, want %#v", got.Fields[0].Default, tt.want)
			}
		})
	}
}

func TestDefault_preserves_absence_and_pointer_optionality(t *testing.T) {
	tests := []struct {
		name            string
		goType          string
		directive       string
		wantDefault     bool
		wantOptionality Optionality
	}{
		{name: "absent value", goType: "bool", wantOptionality: OptionalityOptional},
		{name: "explicit false", goType: "bool", directive: "+fabrica:field:default=false", wantDefault: true, wantOptionality: OptionalityOptional},
		{name: "nillable pointer", goType: "*int", directive: "+fabrica:field:default=0", wantDefault: true, wantOptionality: OptionalityNillable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeDefaultSource(t, tt.goType, tt.directive)

			// When
			got, err := ResolveStorageIntent(filename, "Record", DialectSQLite)
			// Then
			if err != nil {
				t.Fatalf("ResolveStorageIntent() error = %v", err)
			}
			field := got.Fields[0]
			if (field.Default != nil) != tt.wantDefault || field.Optionality != tt.wantOptionality {
				t.Errorf("Default/Optionality = %#v/%v", field.Default, field.Optionality)
			}
		})
	}
}

func TestDefault_rejects_malformed_and_overflowing_values(t *testing.T) {
	tests := []struct {
		name   string
		goType string
		raw    string
	}{
		{name: "malformed bool", goType: "bool", raw: "truthy"},
		{name: "malformed int", goType: "int", raw: "12x"},
		{name: "overflowed int", goType: "int", raw: strconv.FormatUint(math.MaxUint64, 10)},
		{name: "malformed int64", goType: "int64", raw: "9x"},
		{name: "overflowed int64", goType: "int64", raw: "9223372036854775808"},
		{name: "malformed float64", goType: "float64", raw: "1.2.3"},
		{name: "nan", goType: "float64", raw: "NaN"},
		{name: "positive infinity", goType: "float64", raw: "+Inf"},
		{name: "negative infinity", goType: "float64", raw: "-Inf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeDefaultSource(t, tt.goType, "+fabrica:field:default="+tt.raw)

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
			if defaultErr.Filename != filename || defaultErr.Line <= 0 || defaultErr.TypeName != "RecordSpec" || defaultErr.FieldName != "Value" || defaultErr.Directive != "+fabrica:field:default="+tt.raw {
				t.Errorf("DefaultError context = %#v", defaultErr)
			}
		})
	}
}

func TestConflict_rejects_default_with_transform_or_immutable(t *testing.T) {
	tests := []struct {
		name       string
		directives []string
	}{
		{name: "transformed and default", directives: []string{"+fabrica:field:storage=hashed:bcrypt", "+fabrica:field:default=secret"}},
		{name: "immutable and default", directives: []string{"+fabrica:field:immutable", "+fabrica:field:default=pending"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeDefaultSource(t, "string", tt.directives...)

			// When
			got, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)

			// Then
			if got != nil || !errors.Is(err, ErrInvalidDefault) {
				t.Fatalf("ResolveStorageIntent() = %#v, %v", got, err)
			}
			var defaultErr *DefaultError
			if !errors.As(err, &defaultErr) || defaultErr.Kind != DefaultErrorConflict {
				t.Fatalf("DefaultError = %#v, %v", defaultErr, err)
			}
			if defaultErr.Filename != filename || defaultErr.Line <= 0 || defaultErr.Directive != tt.directives[1] {
				t.Errorf("DefaultError context = %#v", defaultErr)
			}
		})
	}
}

func writeDefaultSource(t *testing.T, goType string, directives ...string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "record.go")
	source := "package test\n\n// +fabrica:resource\ntype Record struct { Spec RecordSpec }\n\ntype RecordSpec struct {\n"
	for _, directive := range directives {
		if directive != "" {
			source += "\t// " + directive + "\n"
		}
	}
	source += "\tValue " + goType + " `json:\"value,omitempty\"`\n}\n"
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatalf("Given source file: %v", err)
	}
	return filename
}
