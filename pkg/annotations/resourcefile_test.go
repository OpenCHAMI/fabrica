// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTypesFile writes src to a temp file and returns its path.
func writeTypesFile(t *testing.T, src string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token_types.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	return path
}

const resourceFirstSrc = `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:index:fields=Owner,Slug:unique
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	// +fabrica:field:storage=hashed:sha256
	// +fabrica:field:sensitive
	Value string ` + "`json:\"value\"`" + `

	// +fabrica:field:size=253
	// +fabrica:field:notnull
	Owner string ` + "`json:\"owner\"`" + `

	// +fabrica:field:index=btree:unique
	Slug string ` + "`json:\"slug\"`" + `
}
`

// specFirstSrc is the same resource with the Spec declared BEFORE the resource.
// This ordering used to discard every field annotation.
const specFirstSrc = `package v1

type TokenSpec struct {
	// +fabrica:field:storage=hashed:sha256
	// +fabrica:field:sensitive
	Value string ` + "`json:\"value\"`" + `

	// +fabrica:field:size=253
	// +fabrica:field:notnull
	Owner string ` + "`json:\"owner\"`" + `

	// +fabrica:field:index=btree:unique
	Slug string ` + "`json:\"slug\"`" + `
}

// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:index:fields=Owner,Slug:unique
type Token struct {
	Spec TokenSpec
}
`

func assertCompleteToken(t *testing.T, annots *ResourceAnnotations) {
	t.Helper()

	if !annots.IsResource {
		t.Error("IsResource = false, want true")
	}
	if annots.StorageMode != StorageModeDedicated {
		t.Errorf("StorageMode = %q, want dedicated", annots.StorageMode)
	}
	if len(annots.Indexes) != 1 {
		t.Errorf("got %d composite indexes, want 1", len(annots.Indexes))
	}

	for _, field := range []string{"Value", "Owner", "Slug"} {
		if annots.Fields[field] == nil {
			t.Errorf("field %q annotations missing", field)
		}
	}

	if f := annots.Fields["Owner"]; f != nil && (f.Size != 253 || !f.NotNull) {
		t.Errorf("Owner size/notnull = %d/%v, want 253/true", f.Size, f.NotNull)
	}
}

// TestParseResourceFileDeclarationOrder pins the fix for the declaration-order
// bug: field annotations must survive regardless of whether <Name>Spec is
// declared before or after <Name>.
func TestParseResourceFileDeclarationOrder(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"resource declared first", resourceFirstSrc},
		{"spec declared first", specFirstSrc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			annots, err := ParseResourceFile(writeTypesFile(t, tc.src), "Token")
			if err != nil {
				t.Fatalf("ParseResourceFile: %v", err)
			}

			assertCompleteToken(t, annots)

			if err := Validate(annots); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

// TestParseResourceFileBothOrdersAgree is the sharper form: the two orderings
// must produce the same annotations, not merely both be non-empty.
func TestParseResourceFileBothOrdersAgree(t *testing.T) {
	first, err := ParseResourceFile(writeTypesFile(t, resourceFirstSrc), "Token")
	if err != nil {
		t.Fatalf("ParseResourceFile (resource first): %v", err)
	}
	second, err := ParseResourceFile(writeTypesFile(t, specFirstSrc), "Token")
	if err != nil {
		t.Fatalf("ParseResourceFile (spec first): %v", err)
	}

	if first.IsResource != second.IsResource || first.StorageMode != second.StorageMode {
		t.Errorf("resource-level state differs: %v/%s vs %v/%s",
			first.IsResource, first.StorageMode, second.IsResource, second.StorageMode)
	}
	if len(first.Fields) != len(second.Fields) {
		t.Fatalf("field count differs: %d vs %d", len(first.Fields), len(second.Fields))
	}
	for name, a := range first.Fields {
		b := second.Fields[name]
		if b == nil {
			t.Errorf("field %q missing when the spec is declared first", name)
			continue
		}
		if a.Size != b.Size || a.NotNull != b.NotNull || a.Sensitive != b.Sensitive {
			t.Errorf("field %q differs between orderings", name)
		}
	}
	if len(first.Indexes) != len(second.Indexes) {
		t.Errorf("composite index count differs: %d vs %d", len(first.Indexes), len(second.Indexes))
	}
}

// TestParseFileAnnotationsValidatesDedicatedResource pins the second fix: the
// public API used to return a dedicated resource with zero fields, which always
// failed validation.
func TestParseFileAnnotationsValidatesDedicatedResource(t *testing.T) {
	GetGlobalCache().Clear()

	path := writeTypesFile(t, resourceFirstSrc)

	byType, err := ParseFileAnnotations(path)
	if err != nil {
		t.Fatalf("ParseFileAnnotations: %v", err)
	}

	token := byType["Token"]
	if token == nil {
		t.Fatal("no annotations returned for Token")
	}
	assertCompleteToken(t, token)

	if err := Validate(token); err != nil {
		t.Fatalf("a dedicated resource parsed from file failed validation: %v", err)
	}

	// The Spec entry stays available for callers that relied on it.
	if byType["TokenSpec"] == nil {
		t.Error("TokenSpec entry was dropped; that is a breaking change for existing callers")
	}
}

// TestParseFileAnnotationsMergeSurvivesCache guards the interaction between the
// merge and the parse cache: a second call must return the merged resource too.
func TestParseFileAnnotationsMergeSurvivesCache(t *testing.T) {
	GetGlobalCache().Clear()

	path := writeTypesFile(t, specFirstSrc)

	if _, err := ParseFileAnnotations(path); err != nil {
		t.Fatalf("first ParseFileAnnotations: %v", err)
	}

	cached, err := ParseFileAnnotations(path)
	if err != nil {
		t.Fatalf("second ParseFileAnnotations: %v", err)
	}

	token := cached["Token"]
	if token == nil {
		t.Fatal("no annotations returned for Token on the cached read")
	}
	assertCompleteToken(t, token)

	if err := Validate(token); err != nil {
		t.Fatalf("cached dedicated resource failed validation: %v", err)
	}
}

// TestParseResourceFileReportsAnnotationErrors pins the third fix: a malformed
// annotation used to be swallowed, leaving the caller with a silently
// incomplete resource.
func TestParseResourceFileReportsAnnotationErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name: "bad annotation on the resource",
			src: `package v1

// +fabrica:resource
// +fabrica:storage=bogus
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	Value string
}
`,
			wantErr: "unknown storage mode",
		},
		{
			name: "bad annotation on the spec",
			src: `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	// +fabrica:field:size=999999
	Value string
}
`,
			wantErr: "out of range",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseResourceFile(writeTypesFile(t, tc.src), "Token")
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestParseResourceFileMissingResource keeps the lenient behaviour callers rely
// on: a file that does not declare the resource is not an error.
func TestParseResourceFileMissingResource(t *testing.T) {
	annots, err := ParseResourceFile(writeTypesFile(t, resourceFirstSrc), "Absent")
	if err != nil {
		t.Fatalf("ParseResourceFile: %v", err)
	}
	if annots == nil {
		t.Fatal("expected empty annotations, got nil")
	}
	if annots.IsResource || len(annots.Fields) != 0 {
		t.Errorf("expected empty annotations, got %+v", annots)
	}
}

// TestParseResourceFileSpecOnly covers a spec with no resource declaration.
func TestParseResourceFileSpecOnly(t *testing.T) {
	src := `package v1

type TokenSpec struct {
	// +fabrica:field:sensitive
	Value string
}
`

	annots, err := ParseResourceFile(writeTypesFile(t, src), "Token")
	if err != nil {
		t.Fatalf("ParseResourceFile: %v", err)
	}
	if annots.Fields["Value"] == nil || !annots.Fields["Value"].Sensitive {
		t.Errorf("spec-only field annotations were dropped: %+v", annots.Fields)
	}
	if annots.IsResource {
		t.Error("IsResource = true without a resource declaration")
	}
}
