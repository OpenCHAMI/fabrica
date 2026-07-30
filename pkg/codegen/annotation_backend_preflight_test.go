// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestPrepareResourceAnnotations_rejects_field_directives_when_storage_cannot_enforce_them(t *testing.T) {
	directives := []string{
		"+fabrica:field:storage=hashed:bcrypt:cost=12",
		"+fabrica:field:storage=encrypted:aes256:key=env",
		"+fabrica:field:storage=hashed:argon2:memory=65536",
		"+fabrica:field:storage=hashed:sha256",
		"+fabrica:field:sensitive",
		"+fabrica:field:default=guest",
		"+fabrica:field:unique",
		"+fabrica:field:index",
		"+fabrica:field:immutable",
		"+fabrica:field:storage=default",
	}
	backends := []string{"ent", "file"}

	for _, backend := range backends {
		for _, directive := range directives {
			t.Run(backend+"/"+strings.TrimPrefix(directive, "+fabrica:field:"), func(t *testing.T) {
				// Given
				sourcePath := writeAnnotationSource(t, "token.go", genericTokenSourceWithDirective(directive))
				gen := NewGenerator(t.TempDir(), "main", "example.com/test")
				gen.SetStorageType(backend)
				gen.SetDBDriver("sqlite")
				gen.Resources = []ResourceMetadata{{Name: "Token", SourcePath: sourcePath}}

				// When
				err := gen.PrepareResourceAnnotations()

				// Then
				if !errors.Is(err, annotations.ErrUnsupportedCapability) {
					t.Fatalf("PrepareResourceAnnotations() error = %v, want unsupported capability", err)
				}
				var capabilityErr *annotations.CapabilityError
				if !errors.As(err, &capabilityErr) {
					t.Fatalf("errors.As(*annotations.CapabilityError) = false: %v", err)
				}
				if capabilityErr.Filename != sourcePath || capabilityErr.Line <= 0 || capabilityErr.TypeName != "TokenSpec" || capabilityErr.FieldName != "Value" || capabilityErr.Directive != directive {
					t.Fatalf("CapabilityError = %#v", capabilityErr)
				}
			})
		}
	}
}

func TestPrepareResourceAnnotations_rejects_dedicated_mode_for_file_backend(t *testing.T) {
	// Given
	sourcePath := writeAnnotationSource(t, "token.go", dedicatedTokenSource)
	gen := NewGenerator(t.TempDir(), "main", "example.com/test")
	gen.SetStorageType("file")
	gen.Resources = []ResourceMetadata{{Name: "Token", SourcePath: sourcePath}}

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if !errors.Is(err, annotations.ErrUnsupportedCapability) {
		t.Fatalf("PrepareResourceAnnotations() error = %v, want unsupported capability", err)
	}
	var capabilityErr *annotations.CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Filename != sourcePath || capabilityErr.Line <= 0 || capabilityErr.TypeName != "Token" || capabilityErr.FieldName != "" || capabilityErr.Directive != "+fabrica:storage=dedicated" {
		t.Fatalf("CapabilityError = %#v", capabilityErr)
	}
}

func TestPrepareResourceAnnotations_preserves_generic_mode_without_field_directives(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		source  string
	}{
		{name: "implicit generic Ent", backend: "ent", source: unannotatedTokenSource},
		{name: "explicit generic Ent", backend: "ent", source: explicitGenericTokenSource},
		{name: "implicit generic file", backend: "file", source: unannotatedTokenSource},
		{name: "explicit generic file", backend: "file", source: explicitGenericTokenSource},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			gen := NewGenerator(t.TempDir(), "main", "example.com/test")
			gen.SetStorageType(test.backend)
			gen.Resources = []ResourceMetadata{{Name: "Token", SourcePath: writeAnnotationSource(t, "token.go", test.source)}}

			// When
			err := gen.PrepareResourceAnnotations()

			// Then
			if err != nil {
				t.Fatalf("PrepareResourceAnnotations() error = %v", err)
			}
			if got := gen.Resources[0].Annotations.StorageMode; got != annotations.StorageModeGeneric {
				t.Fatalf("storage mode = %q, want generic", got)
			}
		})
	}
}

func genericTokenSourceWithDirective(directive string) string {
	return fmt.Sprintf(`package fixture

// +fabrica:resource
type Token struct { Spec TokenSpec }

type TokenSpec struct {
	// %s
	Value string
}`, directive)
}

const explicitGenericTokenSource = `package fixture

// +fabrica:resource
// +fabrica:storage=generic
type Token struct { Spec TokenSpec }
type TokenSpec struct { Value string }`
