// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapabilities_postgresql_index_field_type_matrix(t *testing.T) {
	fieldTypes := []struct {
		name   string
		goType string
		kind   FieldKind
	}{
		{name: "string", goType: "string", kind: FieldKindString},
		{name: "bool", goType: "bool", kind: FieldKindBool},
		{name: "int", goType: "int", kind: FieldKindInt},
		{name: "int64", goType: "int64", kind: FieldKindInt64},
		{name: "float64", goType: "float64", kind: FieldKindFloat64},
		{name: "time", goType: "time.Time", kind: FieldKindTime},
		{name: "string slice", goType: "[]string", kind: FieldKindStringSlice},
		{name: "string pointer", goType: "*string", kind: FieldKindString},
		{name: "bool pointer", goType: "*bool", kind: FieldKindBool},
		{name: "int pointer", goType: "*int", kind: FieldKindInt},
		{name: "int64 pointer", goType: "*int64", kind: FieldKindInt64},
		{name: "float64 pointer", goType: "*float64", kind: FieldKindFloat64},
		{name: "time pointer", goType: "*time.Time", kind: FieldKindTime},
	}
	scalarKinds := map[FieldKind]bool{
		FieldKindString: true, FieldKindBool: true, FieldKindInt: true,
		FieldKindInt64: true, FieldKindFloat64: true, FieldKindTime: true,
	}
	allKinds := map[FieldKind]bool{
		FieldKindString: true, FieldKindBool: true, FieldKindInt: true,
		FieldKindInt64: true, FieldKindFloat64: true, FieldKindTime: true,
		FieldKindStringSlice: true,
	}
	indexes := []struct {
		name      string
		directive string
		kind      IndexKind
		allowed   map[FieldKind]bool
	}{
		{name: "btree", directive: "// +fabrica:field:index=btree\n", kind: IndexBTree, allowed: allKinds},
		{name: "gin", directive: "// +fabrica:field:index=gin\n", kind: IndexGIN, allowed: map[FieldKind]bool{FieldKindStringSlice: true}},
		{name: "gist", directive: "// +fabrica:field:index=gist\n", kind: IndexGiST, allowed: map[FieldKind]bool{}},
		{name: "hash", directive: "// +fabrica:field:index=hash\n", kind: IndexHash, allowed: scalarKinds},
	}

	for _, indexCase := range indexes {
		for _, fieldCase := range fieldTypes {
			t.Run(indexCase.name+"/"+fieldCase.name, func(t *testing.T) {
				// Given
				filename := writeCapabilitySource(t, fieldCase.goType, indexCase.directive)

				// When
				got, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)

				// Then
				if indexCase.allowed[fieldCase.kind] {
					if err != nil {
						t.Fatalf("ResolveStorageIntent() error = %v", err)
					}
					if got.Fields[0].Index != indexCase.kind {
						t.Errorf("index = %v, want %v", got.Fields[0].Index, indexCase.kind)
					}
					return
				}
				assertIndexCapabilityError(t, got, err, filename, indexCase.directive)
			})
		}
	}
}

func TestUnsupportedCapabilities_rejects_named_aliases(t *testing.T) {
	tests := []struct {
		name        string
		declaration string
		goType      string
	}{
		{name: "named string", declaration: "type NamedString string", goType: "NamedString"},
		{name: "string alias", declaration: "type StringAlias = string", goType: "StringAlias"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeAliasedFieldSource(t, tt.declaration, tt.goType)

			// When
			got, err := ResolveStorageIntent(filename, "Record", DialectPostgreSQL)

			// Then
			if got != nil || !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("ResolveStorageIntent() = %#v, %v", got, err)
			}
			var capabilityErr *CapabilityError
			if !errors.As(err, &capabilityErr) || capabilityErr.Capability != CapabilityFieldType {
				t.Fatalf("CapabilityError = %#v, %v", capabilityErr, err)
			}
			if capabilityErr.Filename != filename || capabilityErr.Line <= 0 || capabilityErr.TypeName != "RecordSpec" || capabilityErr.FieldName != "Value" || capabilityErr.Directive != "field declaration" {
				t.Errorf("CapabilityError context = %#v", capabilityErr)
			}
		})
	}
}

func assertIndexCapabilityError(t *testing.T, got *ResolvedResourceStorage, err error, filename, directive string) {
	t.Helper()
	if got != nil || !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("ResolveStorageIntent() = %#v, %v", got, err)
	}
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Capability != CapabilityIndex {
		t.Fatalf("CapabilityError = %#v, %v", capabilityErr, err)
	}
	wantDirective := strings.TrimSpace(strings.TrimPrefix(directive, "//"))
	if capabilityErr.Filename != filename || capabilityErr.Line <= 0 || capabilityErr.TypeName != "RecordSpec" || capabilityErr.FieldName != "Value" || capabilityErr.Directive != wantDirective {
		t.Errorf("CapabilityError context = %#v", capabilityErr)
	}
}

func writeAliasedFieldSource(t *testing.T, declaration, goType string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "record.go")
	source := fmt.Sprintf("package test\n\n%s\n\n// +fabrica:resource\ntype Record struct { Spec RecordSpec }\n\ntype RecordSpec struct { Value %s }\n", declaration, goType)
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatalf("Given source file: %v", err)
	}
	return filename
}
