// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSecurityDialect_unsupported_crypto_types_and_indexes_return_capability_errors(t *testing.T) {
	tests := []struct {
		name       string
		goType     string
		directive  string
		dialect    Dialect
		capability CapabilityKind
	}{
		{name: "encryption", goType: "string", directive: "// +fabrica:field:storage=encrypted:aes256:key=env\n", dialect: DialectPostgreSQL, capability: CapabilityTransform},
		{name: "argon2", goType: "string", directive: "// +fabrica:field:storage=hashed:argon2\n", dialect: DialectPostgreSQL, capability: CapabilityTransform},
		{name: "sha256", goType: "string", directive: "// +fabrica:field:storage=hashed:sha256\n", dialect: DialectPostgreSQL, capability: CapabilityTransform},
		{name: "bcrypt non-string", goType: "int", directive: "// +fabrica:field:storage=hashed:bcrypt\n", dialect: DialectPostgreSQL, capability: CapabilityTransform},
		{name: "map", goType: "map[string]string", dialect: DialectPostgreSQL, capability: CapabilityFieldType},
		{name: "nested", goType: "Nested", dialect: DialectPostgreSQL, capability: CapabilityFieldType},
		{name: "named", goType: "NamedString", dialect: DialectPostgreSQL, capability: CapabilityFieldType},
		{name: "sqlite gin", goType: "[]string", directive: "// +fabrica:field:index=gin\n", dialect: DialectSQLite, capability: CapabilityIndex},
		{name: "sqlite gist", goType: "string", directive: "// +fabrica:field:index=gist\n", dialect: DialectSQLite, capability: CapabilityIndex},
		{name: "sqlite hash", goType: "string", directive: "// +fabrica:field:index=hash\n", dialect: DialectSQLite, capability: CapabilityIndex},
		{name: "postgres gin scalar", goType: "string", directive: "// +fabrica:field:index=gin\n", dialect: DialectPostgreSQL, capability: CapabilityIndex},
		{name: "postgres gist scalar", goType: "string", directive: "// +fabrica:field:index=gist\n", dialect: DialectPostgreSQL, capability: CapabilityIndex},
		{name: "postgres hash slice", goType: "[]string", directive: "// +fabrica:field:index=hash\n", dialect: DialectPostgreSQL, capability: CapabilityIndex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			filename := writeSecurityCapabilitySource(t, tt.goType, tt.directive)

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
			if capabilityErr.Capability != tt.capability || capabilityErr.Filename != filename || capabilityErr.Line <= 0 || capabilityErr.Column <= 0 || capabilityErr.TypeName != "RecordSpec" || capabilityErr.FieldName != "Value" || capabilityErr.Directive == "" {
				t.Fatalf("CapabilityError context = %#v", capabilityErr)
			}
		})
	}
}

func TestSecurityDialect_mysql_is_rejected_before_resolution(t *testing.T) {
	// Given
	source := SourcePosition{Filename: "record.go", Line: 8, Column: 1, TypeName: "Record", Directive: "database dialect"}

	// When
	dialect, err := ParseDialect("mysql", source)

	// Then
	if dialect != DialectUnknown || !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("ParseDialect(mysql) = %v, %v", dialect, err)
	}
	var capabilityErr *CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Capability != CapabilityDialect || capabilityErr.Filename != source.Filename || capabilityErr.Line != source.Line || capabilityErr.TypeName != source.TypeName || capabilityErr.Directive != source.Directive {
		t.Fatalf("CapabilityError = %#v", capabilityErr)
	}
}

func writeSecurityCapabilitySource(t *testing.T, goType, directive string) string {
	t.Helper()
	declaration := ""
	if goType == "NamedString" {
		declaration = "type NamedString string\n"
	}
	filename := writeCapabilitySource(t, goType, directive)
	if declaration == "" {
		return filename
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	content = []byte(strings.Replace(string(content), "type Nested struct{}", declaration+"\ntype Nested struct{}", 1))
	if err := os.WriteFile(filename, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}
