// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCapabilities_supports_closed_field_matrix(t *testing.T) {
	types := []struct {
		name   string
		goType string
		kind   FieldKind
		value  reflect.Type
	}{
		{name: "string", goType: "string", kind: FieldKindString, value: reflect.TypeOf("")},
		{name: "bool", goType: "bool", kind: FieldKindBool, value: reflect.TypeOf(false)},
		{name: "int", goType: "int", kind: FieldKindInt, value: reflect.TypeOf(int(0))},
		{name: "int64", goType: "int64", kind: FieldKindInt64, value: reflect.TypeOf(int64(0))},
		{name: "float64", goType: "float64", kind: FieldKindFloat64, value: reflect.TypeOf(float64(0))},
		{name: "time", goType: "time.Time", kind: FieldKindTime, value: reflect.TypeOf(time.Time{})},
		{name: "string slice", goType: "[]string", kind: FieldKindStringSlice, value: reflect.TypeOf([]string{})},
	}
	dialects := []struct {
		name    string
		dialect Dialect
	}{
		{name: "postgresql", dialect: DialectPostgreSQL},
		{name: "sqlite", dialect: DialectSQLite},
	}

	for _, typeCase := range types {
		for _, dialectCase := range dialects {
			for _, index := range []string{"", "// +fabrica:field:index\n"} {
				name := fmt.Sprintf("%s/%s/index=%t", typeCase.name, dialectCase.name, index != "")
				t.Run(name, func(t *testing.T) {
					// Given
					filename := writeCapabilitySource(t, typeCase.goType, index)

					// When
					got, err := ResolveStorageIntent(filename, "Record", dialectCase.dialect)

					// Then
					if err != nil {
						t.Fatalf("ResolveStorageIntent() error = %v", err)
					}
					field := got.Fields[0]
					if field.Type.Kind != typeCase.kind || field.Type.GoType() != typeCase.value {
						t.Errorf("field type = %#v/%v", field.Type, field.Type.GoType())
					}
					if field.Transform.Kind != TransformStandard || field.Dialect != dialectCase.dialect {
						t.Errorf("storage intent = %#v", field)
					}
					if field.JSONName != "storedValue" || field.Source.Filename != filename || field.Source.Line <= 0 {
						t.Errorf("field metadata = %#v", field)
					}
					wantIndex := IndexNone
					if index != "" {
						wantIndex = IndexBTree
					}
					if field.Index != wantIndex {
						t.Errorf("index = %v, want %v", field.Index, wantIndex)
					}
				})
			}
		}
	}
}

func TestCapabilities_supports_ent_nillable_scalar_pointers(t *testing.T) {
	types := []string{"*string", "*bool", "*int", "*int64", "*float64", "*time.Time"}
	for _, goType := range types {
		for _, dialect := range []Dialect{DialectPostgreSQL, DialectSQLite} {
			for _, directive := range []string{"", "// +fabrica:field:index\n"} {
				t.Run(fmt.Sprintf("%s/%d/index=%t", goType, dialect, directive != ""), func(t *testing.T) {
					// Given
					filename := writeCapabilitySource(t, goType, directive)

					// When
					got, err := ResolveStorageIntent(filename, "Record", dialect)

					// Then
					if err != nil {
						t.Fatalf("ResolveStorageIntent() error = %v", err)
					}
					if got.Fields[0].Optionality != OptionalityNillable || got.Fields[0].Type.GoType().Kind() != reflect.Pointer {
						t.Errorf("pointer intent = %#v", got.Fields[0])
					}
				})
			}
		}
	}
}

func TestCapabilities_supports_bcrypt(t *testing.T) {
	tests := []struct {
		name      string
		goType    string
		directive string
		transform TransformKind
		index     IndexKind
		dialect   Dialect
	}{
		{name: "bcrypt string postgresql", goType: "string", directive: "// +fabrica:field:storage=hashed:bcrypt:cost=12\n", transform: TransformBcrypt, index: IndexNone, dialect: DialectPostgreSQL},
		{name: "bcrypt string sqlite", goType: "string", directive: "// +fabrica:field:storage=hashed:bcrypt:cost=12\n", transform: TransformBcrypt, index: IndexNone, dialect: DialectSQLite},
		{name: "bcrypt nillable string", goType: "*string", directive: "// +fabrica:field:storage=hashed:bcrypt:cost=12\n", transform: TransformBcrypt, index: IndexNone, dialect: DialectPostgreSQL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeCapabilitySource(t, tt.goType, tt.directive)

			// When
			got, err := ResolveStorageIntent(filename, "Record", tt.dialect)

			// Then
			if err != nil {
				t.Fatalf("ResolveStorageIntent() error = %v", err)
			}
			if got.Fields[0].Transform.Kind != tt.transform || got.Fields[0].Index != tt.index {
				t.Errorf("intent = %#v", got.Fields[0])
			}
		})
	}
}

func TestUnsupportedCapabilities_return_typed_source_error(t *testing.T) {
	tests := []struct {
		name      string
		goType    string
		directive string
		dialect   Dialect
	}{
		{name: "map", goType: "map[string]string", dialect: DialectPostgreSQL},
		{name: "nested struct", goType: "Nested", dialect: DialectPostgreSQL},
		{name: "arbitrary slice", goType: "[]int", dialect: DialectPostgreSQL},
		{name: "unsupported pointer", goType: "*[]string", dialect: DialectPostgreSQL},
		{name: "encrypted", goType: "string", directive: "// +fabrica:field:storage=encrypted:aes256:key=env\n", dialect: DialectPostgreSQL},
		{name: "argon2", goType: "string", directive: "// +fabrica:field:storage=hashed:argon2\n", dialect: DialectPostgreSQL},
		{name: "sha256", goType: "string", directive: "// +fabrica:field:storage=hashed:sha256\n", dialect: DialectPostgreSQL},
		{name: "bcrypt non-string", goType: "int", directive: "// +fabrica:field:storage=hashed:bcrypt\n", dialect: DialectPostgreSQL},
		{name: "sqlite gin", goType: "string", directive: "// +fabrica:field:index=gin\n", dialect: DialectSQLite},
		{name: "sqlite gist", goType: "string", directive: "// +fabrica:field:index=gist\n", dialect: DialectSQLite},
		{name: "sqlite hash", goType: "string", directive: "// +fabrica:field:index=hash\n", dialect: DialectSQLite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeCapabilitySource(t, tt.goType, tt.directive)

			// When
			got, err := ResolveStorageIntent(filename, "Record", tt.dialect)

			// Then
			if got != nil || !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("ResolveStorageIntent() = %#v, %v", got, err)
			}
			var capabilityErr *CapabilityError
			if !errors.As(err, &capabilityErr) {
				t.Fatalf("errors.As(*CapabilityError) = false: %v", err)
			}
			if capabilityErr.Filename != filename || capabilityErr.Line <= 0 || capabilityErr.TypeName != "RecordSpec" || capabilityErr.FieldName != "Value" || capabilityErr.Directive == "" {
				t.Errorf("CapabilityError context = %#v", capabilityErr)
			}
		})
	}
}

func TestUnsupportedCapabilities_rejects_mysql_dialect(t *testing.T) {
	// Given
	source := SourcePosition{Filename: "record.go", Line: 4, TypeName: "RecordSpec", FieldName: "Value", Directive: "mysql"}

	// When
	got, err := ParseDialect("mysql", source)

	// Then
	if got != DialectUnknown || !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("ParseDialect() = %v, %v", got, err)
	}
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Filename != source.Filename || capabilityErr.Directive != source.Directive {
		t.Errorf("CapabilityError = %#v", capabilityErr)
	}
}

func writeCapabilitySource(t *testing.T, goType, directive string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "record.go")
	source := "package test\n\nimport \"time\"\n\ntype Nested struct{}\n\n// +fabrica:resource\n// +fabrica:storage=dedicated\ntype Record struct { Spec RecordSpec }\n\ntype RecordSpec struct {\n\t" + directive + "\tValue " + goType + " `json:\"storedValue,omitempty\"`\n}\n"
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatalf("Given source file: %v", err)
	}
	return filename
}
