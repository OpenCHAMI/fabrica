// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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

func TestGeneratedAnnotationProject_backend_preflight_failure_preserves_managed_output(t *testing.T) {
	tests := []struct {
		name     string
		backend  string
		source   string
		expected []string
	}{
		{name: "generic Ent field directive", backend: "ent", source: generatedGenericTokenSourceWithDirective("+fabrica:field:sensitive"), expected: []string{"TokenSpec", "Value", "+fabrica:field:sensitive"}},
		{name: "generic file field directive", backend: "file", source: generatedGenericTokenSourceWithDirective("+fabrica:field:sensitive"), expected: []string{"TokenSpec", "Value", "+fabrica:field:sensitive"}},
		{name: "dedicated file mode", backend: "file", source: validAnnotatedTokenSource, expected: []string{"Token", "+fabrica:storage=dedicated"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, test.backend)
			project.writeResourceSource(t, generatedUnannotatedTokenSource)
			if result := project.generate(t); result.err != nil {
				t.Fatalf("prepare baseline: %s", result.failureMessage())
			}
			project.writeResourceSource(t, test.source)
			before := snapshotGeneratedManagedTree(t, project.root)

			// When
			result := project.generate(t)

			// Then
			if result.err == nil {
				t.Fatalf("generation succeeded for unsupported %s storage contract", test.backend)
			}
			for _, expected := range append([]string{"token_types.go"}, test.expected...) {
				if !strings.Contains(result.stdout+result.stderr, expected) {
					t.Errorf("failure missing %q\n%s", expected, result.failureMessage())
				}
			}
			after := snapshotGeneratedManagedTree(t, project.root)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("preflight failure mutated managed output\nbefore: %#v\nafter: %#v", before, after)
			}
		})
	}
}

func TestGeneratedAnnotationProject_generic_without_field_directives_builds(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		source  string
	}{
		{name: "implicit generic Ent", backend: "ent", source: generatedUnannotatedTokenSource},
		{name: "explicit generic Ent", backend: "ent", source: generatedExplicitGenericTokenSource},
		{name: "implicit generic file", backend: "file", source: generatedUnannotatedTokenSource},
		{name: "explicit generic file", backend: "file", source: generatedExplicitGenericTokenSource},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			project := newGeneratedProject(t, test.backend)
			project.writeResourceSource(t, test.source)

			// When
			result := project.generate(t)
			if result.err == nil {
				result = project.tidy(t)
			}
			if result.err == nil && test.backend == "ent" {
				result = project.run(t, "generic-ent-codegen", project.root, "go", "generate", "./internal/storage")
			}
			if result.err == nil {
				result = project.build(t)
			}

			// Then
			if result.err != nil {
				t.Fatalf("%s", result.failureMessage())
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

func generatedGenericTokenSourceWithDirective(directive string) string {
	return strings.Replace(
		generatedUnannotatedTokenSource,
		"type TokenSpec struct { Value string }",
		fmt.Sprintf("type TokenSpec struct {\n\t// %s\n\tValue string\n}", directive),
		1,
	)
}

var generatedExplicitGenericTokenSource = strings.Replace(
	generatedUnannotatedTokenSource,
	"type Token struct {",
	"// +fabrica:resource\n// +fabrica:storage=generic\ntype Token struct {",
	1,
)

func snapshotGeneratedManagedTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	for _, relativeRoot := range []string{"internal", "pkg/resources"} {
		for path, content := range snapshotAtomicTree(t, filepath.Join(root, relativeRoot)) {
			result[filepath.Join(relativeRoot, path)] = content
		}
	}
	return result
}
