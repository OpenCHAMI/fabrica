// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"strings"
	"testing"
)

// parseSource runs the real AST path over a source snippet, mirroring how
// codegen.Generator.ParseResourceAnnotations resolves a resource: type-level
// annotations come from <Name>, field-level annotations from <Name>Spec.
func parseSource(t *testing.T, src, typeName string) (*ResourceAnnotations, error) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "types.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}

	var result *ResourceAnnotations
	specTypeName := typeName + "Spec"
	specFields := make(map[string]*FieldAnnotations)
	specFieldNames := make(map[string]bool)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			switch typeSpec.Name.Name {
			case typeName:
				annots, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
				if err != nil {
					return nil, err
				}
				result = annots

			case specTypeName:
				annots, err := ParseResourceAnnotations(typeSpec, genDecl.Doc)
				if err != nil {
					return nil, err
				}
				maps.Copy(specFields, annots.Fields)
				maps.Copy(specFieldNames, annots.SpecFields)
			}
		}
	}

	if result == nil {
		t.Fatalf("type %q not found in source", typeName)
	}

	maps.Copy(result.Fields, specFields)
	maps.Copy(result.SpecFields, specFieldNames)

	return result, nil
}

func dedicatedSrc(typeAnnots, fieldAnnots string) string {
	return `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
` + typeAnnots + `
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	` + fieldAnnots + `
	Value string ` + "`json:\"value\"`" + `
	Owner string ` + "`json:\"owner\"`" + `
}
`
}

// --- Composite indexes -------------------------------------------------------

func TestParseCompositeIndex(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		wantFields []string
		wantName   string
		wantUnique bool
		wantType   IndexType
	}{
		{
			name:       "minimal",
			annotation: "// +fabrica:index:fields=Value,Owner",
			wantFields: []string{"Value", "Owner"},
			wantType:   IndexTypeBTree,
		},
		{
			name:       "named unique",
			annotation: "// +fabrica:index:fields=Value,Owner:name=idx_value_owner:unique",
			wantFields: []string{"Value", "Owner"},
			wantName:   "idx_value_owner",
			wantUnique: true,
			wantType:   IndexTypeBTree,
		},
		{
			name:       "explicit type",
			annotation: "// +fabrica:index:fields=Value,Owner:type=gin",
			wantFields: []string{"Value", "Owner"},
			wantType:   IndexTypeGIN,
		},
		{
			name:       "whitespace in field list is trimmed",
			annotation: "// +fabrica:index:fields=Value, Owner",
			wantFields: []string{"Value", "Owner"},
			wantType:   IndexTypeBTree,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annots, err := parseSource(t, dedicatedSrc(tt.annotation, ""), "Token")
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if len(annots.Indexes) != 1 {
				t.Fatalf("got %d composite indexes, want 1", len(annots.Indexes))
			}

			idx := annots.Indexes[0]
			if strings.Join(idx.Fields, ",") != strings.Join(tt.wantFields, ",") {
				t.Errorf("Fields = %v, want %v", idx.Fields, tt.wantFields)
			}
			if idx.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", idx.Name, tt.wantName)
			}
			if idx.Unique != tt.wantUnique {
				t.Errorf("Unique = %v, want %v", idx.Unique, tt.wantUnique)
			}
			if idx.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", idx.Type, tt.wantType)
			}
		})
	}
}

func TestParseCompositeIndexErrors(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		wantErr    string
	}{
		{"no fields", "// +fabrica:index:name=idx_foo", "requires fields="},
		{"bad type", "// +fabrica:index:fields=Value,Owner:type=bogus", "unknown index type"},
		{"unknown param", "// +fabrica:index:fields=Value,Owner:sorted", "unknown composite index parameter"},
		{"invalid field name", "// +fabrica:index:fields=owner_id,CreatedAt", "not a valid Go field name"},
		{"invalid index name", "// +fabrica:index:fields=Value,Owner:name=bad-name", "invalid index name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSource(t, dedicatedSrc(tt.annotation, ""), "Token")
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseRejectsUnknownAndTrailingAnnotations(t *testing.T) {
	for _, tt := range []struct {
		name       string
		annotation string
		wantErr    string
	}{
		{"unknown resource directive", "// +fabrica:migraton=additive-only", "unknown resource annotation"},
		{"resource trailing parameter", "// +fabrica:resource:extra", "does not accept parameters"},
		{"storage trailing parameter", "// +fabrica:storage=dedicated:extra", "does not accept trailing parameters"},
		{"migration trailing parameter", "// +fabrica:migration=additive-only:extra", "does not accept trailing parameters"},
		{"unknown field directive", "// +fabrica:field:notnul", "unknown field annotation"},
		{"nullable trailing parameter", "// +fabrica:field:nullable:junk", "does not accept parameters"},
		{"size trailing parameter", "// +fabrica:field:size=10:junk", "does not accept trailing parameters"},
		{"default empty value", "// +fabrica:field:default=", "requires a non-empty value"},
		{"default trailing parameter", "// +fabrica:field:default=pending:junk", "does not accept trailing parameters"},
		{"superseded cascade directive", "// +fabrica:field:cascade=delete", "unknown field annotation"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var src string
			if strings.Contains(tt.annotation, "+fabrica:field:") {
				src = dedicatedSrc("", tt.annotation)
			} else {
				src = dedicatedSrc(tt.annotation, "")
			}

			_, err := parseSource(t, src, "Token")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestParseRejectsUnknownStorageParameters(t *testing.T) {
	for _, tt := range []struct {
		name       string
		annotation string
		wantErr    string
	}{
		{"bcrypt unknown parameter", "// +fabrica:field:storage=hashed:bcrypt:memory=1024", "unknown bcrypt parameter"},
		{"bcrypt duplicate cost", "// +fabrica:field:storage=hashed:bcrypt:cost=10:cost=11", "duplicate bcrypt cost"},
		{"argon2 unknown parameter", "// +fabrica:field:storage=hashed:argon2:cost=12", "unknown argon2 parameter"},
		{"sha256 parameter", "// +fabrica:field:storage=hashed:sha256:cost=12", "sha256 does not accept parameters"},
		{"encryption unknown parameter", "// +fabrica:field:storage=encrypted:aes256:keys=vault", "unknown encryption parameter"},
		{"encryption duplicate key", "// +fabrica:field:storage=encrypted:aes256:key=env:key=vault", "duplicate encryption key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSource(t, dedicatedSrc("", tt.annotation), "Token")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGroupedFieldAnnotationsApplyToEachName(t *testing.T) {
	src := `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	// +fabrica:field:size=64
	OwnerID, DeviceID string ` + "`json:\"owner_id\"`" + `
}
`

	annots, err := parseSource(t, src, "Token")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	for _, name := range []string{"OwnerID", "DeviceID"} {
		field := annots.Fields[name]
		if field == nil || field.Size != 64 || field.FieldType != "string" {
			t.Fatalf("%s annotations = %+v", name, field)
		}
	}
}

func TestValidateCompositeIndex(t *testing.T) {
	tests := []struct {
		name    string
		build   func() *ResourceAnnotations
		wantErr string
	}{
		{
			name: "single column rejected with guidance",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{{Fields: []string{"Value"}, Type: IndexTypeBTree}}
				return a
			},
			wantErr: "use +fabrica:field:index",
		},
		{
			name: "duplicate column rejected",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{{Fields: []string{"Value", "Value"}, Type: IndexTypeBTree}}
				return a
			},
			wantErr: "more than once",
		},
		{
			name: "duplicate index name rejected",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{
					{Fields: []string{"Value", "Owner"}, Name: "idx_a", Type: IndexTypeBTree},
					{Fields: []string{"Owner", "Value"}, Name: "idx_a", Type: IndexTypeBTree},
				}
				return a
			},
			wantErr: "duplicate index name",
		},
		{
			name: "generic storage rejected",
			build: func() *ResourceAnnotations {
				a := NewResourceAnnotations()
				a.IsResource = true
				a.StorageMode = StorageModeGeneric
				a.Indexes = []*CompositeIndex{{Fields: []string{"Value", "Owner"}, Type: IndexTypeBTree}}
				return a
			},
			wantErr: "require +fabrica:storage=dedicated",
		},
		{
			name: "valid composite index accepted",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{{Fields: []string{"Value", "Owner"}, Type: IndexTypeBTree}}
				return a
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.build())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRejectsMalformedCompositeIndexStructs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		indexes []*CompositeIndex
		wantErr string
	}{
		{
			name:    "nil composite index",
			indexes: []*CompositeIndex{nil},
			wantErr: "must not be nil",
		},
		{
			name:    "unknown composite index type",
			indexes: []*CompositeIndex{{Fields: []string{"Value", "Owner"}, Type: IndexType("bogus")}},
			wantErr: "unknown index type",
		},
		{
			name:    "invalid composite field name",
			indexes: []*CompositeIndex{{Fields: []string{"owner_id", "CreatedAt"}, Type: IndexTypeBTree}},
			wantErr: "not a valid Go field name",
		},
		{
			name:    "unknown composite field name",
			indexes: []*CompositeIndex{{Fields: []string{"Value", "Missing"}, Type: IndexTypeBTree}},
			wantErr: "unknown Spec field",
		},
		{
			name:    "invalid composite index name",
			indexes: []*CompositeIndex{{Fields: []string{"Value", "Owner"}, Name: "bad-name", Type: IndexTypeBTree}},
			wantErr: "invalid index name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := dedicatedResource()
			a.Indexes = tt.indexes

			err := Validate(a)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateRejectsDuplicateIndexNamesAcrossCompositeAndFields(t *testing.T) {
	tests := []struct {
		name  string
		build func() *ResourceAnnotations
	}{
		{
			name: "duplicate field index names",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Fields["Value"].Index = &IndexConfig{Type: IndexTypeBTree, Name: "idx_token_lookup"}
				owner := NewFieldAnnotations("Owner")
				owner.Index = &IndexConfig{Type: IndexTypeBTree, Name: "idx_token_lookup"}
				a.Fields["Owner"] = owner
				return a
			},
		},
		{
			name: "field index name duplicates composite index name",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Fields["Value"].Index = &IndexConfig{Type: IndexTypeBTree, Name: "idx_token_lookup"}
				a.Indexes = []*CompositeIndex{{Fields: []string{"Value", "Owner"}, Name: "idx_token_lookup", Type: IndexTypeBTree}}
				return a
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.build())
			if err == nil || !strings.Contains(err.Error(), "duplicate index name") {
				t.Fatalf("expected a duplicate index name error, got %v", err)
			}
		})
	}
}

func TestValidateRejectsUnknownStorageMode(t *testing.T) {
	a := dedicatedResource()
	a.StorageMode = StorageMode("bogus")

	err := Validate(a)
	if err == nil || !strings.Contains(err.Error(), "unknown storage mode") {
		t.Fatalf("expected an unknown storage mode error, got %v", err)
	}
}

// dedicatedResource returns a dedicated-mode resource with one annotated field,
// which satisfies validateDedicatedStorage.
func dedicatedResource() *ResourceAnnotations {
	a := NewResourceAnnotations()
	a.IsResource = true
	a.StorageMode = StorageModeDedicated
	a.Fields["Value"] = NewFieldAnnotations("Value")
	a.SpecFields["Value"] = true
	a.SpecFields["Owner"] = true
	return a
}

// --- Nullability, size -------------------------------------------------------

func TestParseNullabilityAndSize(t *testing.T) {
	src := dedicatedSrc("", `// +fabrica:field:nullable
	// +fabrica:field:size=253`)

	annots, err := parseSource(t, src, "Token")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	f := annots.Fields["Value"]
	if f == nil {
		t.Fatal("expected annotations on field Value")
	}
	if !f.Nullable {
		t.Error("Nullable = false, want true")
	}
	if f.NotNull {
		t.Error("NotNull = true, want false")
	}
	if f.Size != 253 {
		t.Errorf("Size = %d, want 253", f.Size)
	}
}

func TestSizeOutOfRange(t *testing.T) {
	_, err := parseSource(t, dedicatedSrc("", "// +fabrica:field:size=0"), "Token")
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected an out-of-range error, got %v", err)
	}
}

func TestNullableAndNotNullConflict(t *testing.T) {
	a := dedicatedResource()
	f := NewFieldAnnotations("Value")
	f.Nullable = true
	f.NotNull = true
	a.Fields["Value"] = f

	err := Validate(a)
	if err == nil || !strings.Contains(err.Error(), "both nullable and notnull") {
		t.Fatalf("expected a nullable/notnull conflict error, got %v", err)
	}
}

func TestDedicatedOnlyVocabularyRejectedInGenericMode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*FieldAnnotations)
		want  string
	}{
		{"nullable", func(f *FieldAnnotations) { f.Nullable = true }, "nullable requires"},
		{"notnull", func(f *FieldAnnotations) { f.NotNull = true }, "notnull requires"},
		{"size", func(f *FieldAnnotations) { f.Size = 64 }, "size requires"},
		{"relation", func(f *FieldAnnotations) {
			f.Relation = &RelationConfig{Kind: RelationBelongsTo, Target: "User", OnDelete: OnDeleteRestrict}
		}, "relation requires"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewResourceAnnotations()
			a.IsResource = true
			a.StorageMode = StorageModeGeneric
			f := NewFieldAnnotations("Value")
			tc.apply(f)
			a.Fields["Value"] = f

			err := Validate(a)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// --- Relations ---------------------------------------------------------------

func TestParseRelation(t *testing.T) {
	tests := []struct {
		name         string
		annotation   string
		wantKind     RelationKind
		wantTarget   string
		wantOnDelete OnDeleteAction
	}{
		{
			name:         "belongs-to defaults to restrict",
			annotation:   "// +fabrica:field:relation=belongs-to:User",
			wantKind:     RelationBelongsTo,
			wantTarget:   "User",
			wantOnDelete: OnDeleteRestrict,
		},
		{
			name:         "belongs-to with cascade",
			annotation:   "// +fabrica:field:relation=belongs-to:User:on-delete=cascade",
			wantKind:     RelationBelongsTo,
			wantTarget:   "User",
			wantOnDelete: OnDeleteCascade,
		},
		{
			name:         "has-many with set-null",
			annotation:   "// +fabrica:field:relation=has-many:Session:on-delete=set-null",
			wantKind:     RelationHasMany,
			wantTarget:   "Session",
			wantOnDelete: OnDeleteSetNull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annots, err := parseSource(t, dedicatedSrc("", tt.annotation), "Token")
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			rel := annots.Fields["Value"].Relation
			if rel == nil {
				t.Fatal("expected a relation on field Value")
			}
			if rel.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", rel.Kind, tt.wantKind)
			}
			if rel.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", rel.Target, tt.wantTarget)
			}
			if rel.OnDelete != tt.wantOnDelete {
				t.Errorf("OnDelete = %q, want %q", rel.OnDelete, tt.wantOnDelete)
			}
		})
	}
}

func TestParseRelationErrors(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		wantErr    string
	}{
		{"unknown kind", "// +fabrica:field:relation=owns:User", "unknown relation kind"},
		{"missing target", "// +fabrica:field:relation=belongs-to", "requires a target"},
		{"bad on-delete", "// +fabrica:field:relation=belongs-to:User:on-delete=explode", "unknown on-delete action"},
		{"unknown param", "// +fabrica:field:relation=belongs-to:User:eager", "unknown relation parameter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSource(t, dedicatedSrc("", tt.annotation), "Token")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRelationSetNullConflicts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(*FieldAnnotations)
		want  string
	}{
		{"notnull", func(f *FieldAnnotations) { f.NotNull = true }, "conflicts with +fabrica:field:notnull"},
		{"immutable", func(f *FieldAnnotations) { f.Immutable = true }, "conflicts with +fabrica:field:immutable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := dedicatedResource()
			f := NewFieldAnnotations("Value")
			f.Relation = &RelationConfig{Kind: RelationBelongsTo, Target: "User", OnDelete: OnDeleteSetNull}
			tc.apply(f)
			a.Fields["Value"] = f

			err := Validate(a)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRelationSetNullRequiresNullable(t *testing.T) {
	a := dedicatedResource()
	f := NewFieldAnnotations("Value")
	f.Relation = &RelationConfig{Kind: RelationBelongsTo, Target: "User", OnDelete: OnDeleteSetNull}
	a.Fields["Value"] = f

	err := Validate(a)
	if err == nil || !strings.Contains(err.Error(), "requires +fabrica:field:nullable") {
		t.Fatalf("expected set-null to require nullable, got %v", err)
	}

	f.Nullable = true
	if err := Validate(a); err != nil {
		t.Fatalf("expected nullable set-null relation to validate, got %v", err)
	}
}

func TestRelationInvalidTarget(t *testing.T) {
	a := dedicatedResource()
	f := NewFieldAnnotations("Value")
	f.Relation = &RelationConfig{Kind: RelationBelongsTo, Target: "not-a-type", OnDelete: OnDeleteRestrict}
	a.Fields["Value"] = f

	err := Validate(a)
	if err == nil || !strings.Contains(err.Error(), "not a valid Go type name") {
		t.Fatalf("expected an invalid-target error, got %v", err)
	}
}

func TestRelationInvalidTargetRejectsKeywordAndUnexportedName(t *testing.T) {
	for _, target := range []string{"type", "func", "user"} {
		t.Run(target, func(t *testing.T) {
			a := dedicatedResource()
			f := NewFieldAnnotations("Value")
			f.Relation = &RelationConfig{Kind: RelationBelongsTo, Target: target, OnDelete: OnDeleteRestrict}
			a.Fields["Value"] = f

			err := Validate(a)
			if err == nil || !strings.Contains(err.Error(), "not a valid Go type name") {
				t.Fatalf("expected an invalid-target error, got %v", err)
			}
		})
	}
}

func TestValidateRejectsMalformedRelationStructs(t *testing.T) {
	for _, tt := range []struct {
		name     string
		relation *RelationConfig
		wantErr  string
	}{
		{
			name:     "unknown kind",
			relation: &RelationConfig{Kind: RelationKind("owns"), Target: "User", OnDelete: OnDeleteRestrict},
			wantErr:  "unknown relation kind",
		},
		{
			name:     "unknown on-delete",
			relation: &RelationConfig{Kind: RelationBelongsTo, Target: "User", OnDelete: OnDeleteAction("explode")},
			wantErr:  "unknown on-delete action",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := dedicatedResource()
			f := NewFieldAnnotations("Value")
			f.Relation = tt.relation
			a.Fields["Value"] = f

			err := Validate(a)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

// --- Migration policy --------------------------------------------------------

func TestParseMigrationPolicy(t *testing.T) {
	annots, err := parseSource(t, dedicatedSrc("// +fabrica:migration=additive-only", ""), "Token")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if annots.Migration != MigrationPolicyAdditiveOnly {
		t.Errorf("Migration = %q, want %q", annots.Migration, MigrationPolicyAdditiveOnly)
	}
}

func TestMigrationPolicyDefaultsToUnrestricted(t *testing.T) {
	annots, err := parseSource(t, dedicatedSrc("", ""), "Token")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if annots.Migration != MigrationPolicyUnrestricted {
		t.Errorf("Migration = %q, want %q", annots.Migration, MigrationPolicyUnrestricted)
	}
}

func TestMigrationPolicyUnknownValue(t *testing.T) {
	_, err := parseSource(t, dedicatedSrc("// +fabrica:migration=yolo", ""), "Token")
	if err == nil || !strings.Contains(err.Error(), "unknown migration policy") {
		t.Fatalf("expected an unknown-policy error, got %v", err)
	}
}

func TestValidateRejectsUnknownMigrationPolicy(t *testing.T) {
	a := dedicatedResource()
	a.Migration = MigrationPolicy("yolo")

	err := Validate(a)
	if err == nil || !strings.Contains(err.Error(), "unknown migration policy") {
		t.Fatalf("expected an unknown-policy error, got %v", err)
	}
}

// --- Field-level index modifiers ---------------------------------------------

func TestParseIndexModifiers(t *testing.T) {
	src := dedicatedSrc("", "// +fabrica:field:index=btree:unique:name=idx_value")

	annots, err := parseSource(t, src, "Token")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	idx := annots.Fields["Value"].Index
	if idx == nil {
		t.Fatal("expected an index config on field Value")
	}
	if !idx.Unique {
		t.Error("Index.Unique = false, want true")
	}
	if idx.Name != "idx_value" {
		t.Errorf("Index.Name = %q, want %q", idx.Name, "idx_value")
	}
	if idx.Type != IndexTypeBTree {
		t.Errorf("Index.Type = %q, want btree", idx.Type)
	}
}

func TestParseIndexUnknownModifier(t *testing.T) {
	_, err := parseSource(t, dedicatedSrc("", "// +fabrica:field:index=btree:clustered"), "Token")
	if err == nil || !strings.Contains(err.Error(), "unknown index modifier") {
		t.Fatalf("expected an unknown-modifier error, got %v", err)
	}
}

func TestValidateRejectsMalformedFieldAnnotationStructs(t *testing.T) {
	tests := []struct {
		name    string
		apply   func(*ResourceAnnotations)
		wantErr string
	}{
		{
			name: "nil field annotations",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"] = nil
			},
			wantErr: "must not be nil",
		},
		{
			name: "unknown field index type",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].Index = &IndexConfig{Type: IndexType("bogus")}
			},
			wantErr: "unknown index type",
		},
		{
			name: "invalid field index name",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].Index = &IndexConfig{Type: IndexTypeBTree, Name: "bad-name"}
			},
			wantErr: "invalid index name",
		},
		{
			name: "out-of-range size",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].Size = 65536
			},
			wantErr: "field size must be 1-65535",
		},
		{
			name: "size on non-string field",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].FieldType = "int"
				a.Fields["Value"].Size = 64
			},
			wantErr: "size requires a string field",
		},
		{
			name: "hashed storage on non-string field",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].FieldType = "int"
				a.Fields["Value"].Storage = &StorageConfig{Type: StorageTypeHashed, Hash: &HashConfig{Algorithm: HashAlgorithmBcrypt, Cost: 12}}
			},
			wantErr: "hashed storage requires a string field",
		},
		{
			name: "default storage with hash config",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].Storage = &StorageConfig{Type: StorageTypeDefault, Hash: &HashConfig{Algorithm: HashAlgorithmBcrypt, Cost: 12}}
			},
			wantErr: "default storage must not include hash or encryption config",
		},
		{
			name: "hashed storage with encryption config",
			apply: func(a *ResourceAnnotations) {
				a.Fields["Value"].Storage = &StorageConfig{
					Type:       StorageTypeHashed,
					Hash:       &HashConfig{Algorithm: HashAlgorithmBcrypt, Cost: 12},
					Encryption: &EncryptionConfig{Algorithm: "aes256", KeySource: "env"},
				}
			},
			wantErr: "hashed storage must not include encryption config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := dedicatedResource()
			tt.apply(a)

			err := Validate(a)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateRejectsNilAnnotations(t *testing.T) {
	if err := Validate(nil); err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("expected Validate(nil) to fail, got %v", err)
	}
	if err := ValidateForDatabase(nil, "postgres"); err == nil || !strings.Contains(err.Error(), "must not be nil") {
		t.Fatalf("expected ValidateForDatabase(nil) to fail, got %v", err)
	}
}

// --- Per-database validation -------------------------------------------------

func TestCompositeIndexDatabaseCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		indexType IndexType
		driver    string
		wantErr   bool
	}{
		{"gin on sqlite rejected", IndexTypeGIN, "sqlite3", true},
		{"gist on mysql rejected", IndexTypeGiST, "mysql", true},
		{"gin on postgres allowed", IndexTypeGIN, "postgres", false},
		{"btree on sqlite allowed", IndexTypeBTree, "sqlite3", false},
		{"unknown driver rejected", IndexTypeBTree, "sqlitee", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := dedicatedResource()
			a.Indexes = []*CompositeIndex{{Fields: []string{"Value", "Owner"}, Type: tt.indexType}}

			err := ValidateForDatabase(a, tt.driver)
			if tt.wantErr && err == nil {
				t.Fatal("expected a database compatibility error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestDocumentedWorkedExample parses the Session example from README.md so the
// documentation cannot drift away from what the parser actually accepts.
func TestDocumentedWorkedExample(t *testing.T) {
	src := `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:migration=additive-only
// +fabrica:index:fields=OwnerID,CreatedAt:name=idx_session_owner_created
// +fabrica:index:fields=OwnerID,DeviceID:unique
type Session struct {
	Spec SessionSpec
}

type SessionSpec struct {
	// +fabrica:field:storage=hashed:sha256
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Token string ` + "`json:\"token\"`" + `

	// +fabrica:field:relation=belongs-to:User:on-delete=cascade
	// +fabrica:field:notnull
	// +fabrica:field:size=36
	OwnerID string ` + "`json:\"owner_id\"`" + `

	// +fabrica:field:size=128
	// +fabrica:field:notnull
	DeviceID string ` + "`json:\"device_id\"`" + `

	// +fabrica:field:nullable
	// +fabrica:field:size=1024
	UserAgent string ` + "`json:\"user_agent\"`" + `

	// +fabrica:field:index
	CreatedAt string ` + "`json:\"created_at\"`" + `
}
`

	annots, err := parseSource(t, src, "Session")
	if err != nil {
		t.Fatalf("README example failed to parse: %v", err)
	}
	if err := Validate(annots); err != nil {
		t.Fatalf("README example failed to validate: %v", err)
	}
	if err := ValidateForDatabase(annots, "postgres"); err != nil {
		t.Fatalf("README example failed postgres validation: %v", err)
	}

	if len(annots.Indexes) != 2 {
		t.Fatalf("got %d composite indexes, want 2", len(annots.Indexes))
	}
	if annots.Indexes[0].Name != "idx_session_owner_created" {
		t.Errorf("first index name = %q", annots.Indexes[0].Name)
	}
	if !annots.Indexes[1].Unique {
		t.Error("second index should be unique")
	}
	if annots.Migration != MigrationPolicyAdditiveOnly {
		t.Errorf("Migration = %q, want additive-only", annots.Migration)
	}

	owner := annots.Fields["OwnerID"]
	if owner.Relation == nil || owner.Relation.OnDelete != OnDeleteCascade {
		t.Fatalf("OwnerID relation = %+v", owner.Relation)
	}
	if owner.Size != 36 || !owner.NotNull {
		t.Errorf("OwnerID size/notnull = %d/%v, want 36/true", owner.Size, owner.NotNull)
	}

	if ua := annots.Fields["UserAgent"]; !ua.Nullable || ua.Size != 1024 {
		t.Errorf("UserAgent nullable/size = %v/%d, want true/1024", ua.Nullable, ua.Size)
	}
}

// --- Backward compatibility --------------------------------------------------

// TestExistingVocabularyUnchanged pins every annotation that existed before the
// vocabulary was extended, so a future change to the parser cannot quietly
// alter their meaning.
func TestExistingVocabularyUnchanged(t *testing.T) {
	src := `package v1

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
	Spec TokenSpec
}

type TokenSpec struct {
	// +fabrica:field:storage=hashed:bcrypt:cost=14
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Value string ` + "`json:\"value\"`" + `

	// +fabrica:field:storage=encrypted:aes256:key=vault
	Secret string ` + "`json:\"secret\"`" + `

	// +fabrica:field:index=gin
	// +fabrica:field:unique
	// +fabrica:field:default=pending
	Status string ` + "`json:\"status\"`" + `
}
`

	annots, err := parseSource(t, src, "Token")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if !annots.IsResource {
		t.Error("IsResource = false, want true")
	}
	if annots.StorageMode != StorageModeDedicated {
		t.Errorf("StorageMode = %q, want dedicated", annots.StorageMode)
	}

	value := annots.Fields["Value"]
	if value.Storage == nil || value.Storage.Type != StorageTypeHashed {
		t.Fatalf("Value storage = %+v, want hashed", value.Storage)
	}
	if value.Storage.Hash.Algorithm != HashAlgorithmBcrypt {
		t.Errorf("Value hash algorithm = %q, want bcrypt", value.Storage.Hash.Algorithm)
	}
	if value.Storage.Hash.Cost != 14 {
		t.Errorf("Value bcrypt cost = %d, want 14", value.Storage.Hash.Cost)
	}
	if !value.Sensitive || !value.Immutable {
		t.Errorf("Value sensitive/immutable = %v/%v, want true/true", value.Sensitive, value.Immutable)
	}

	secret := annots.Fields["Secret"]
	if secret.Storage == nil || secret.Storage.Type != StorageTypeEncrypted {
		t.Fatalf("Secret storage = %+v, want encrypted", secret.Storage)
	}
	if secret.Storage.Encryption.Algorithm != "aes256" {
		t.Errorf("Secret algorithm = %q, want aes256", secret.Storage.Encryption.Algorithm)
	}
	if secret.Storage.Encryption.KeySource != "vault" {
		t.Errorf("Secret key source = %q, want vault", secret.Storage.Encryption.KeySource)
	}

	status := annots.Fields["Status"]
	if status.Index == nil || status.Index.Type != IndexTypeGIN {
		t.Fatalf("Status index = %+v, want gin", status.Index)
	}
	if status.Index.Unique {
		t.Error("Status index Unique = true; a bare index= must not imply a unique index")
	}
	if !status.Unique {
		t.Error("Status Unique = false, want true")
	}
	if status.Default != "pending" {
		t.Errorf("Status Default = %q, want pending", status.Default)
	}

	// The new fields must stay at their zero values when unannotated.
	if status.Nullable || status.NotNull || status.Size != 0 || status.Relation != nil {
		t.Error("new vocabulary leaked onto a field that does not declare it")
	}
	if len(annots.Indexes) != 0 {
		t.Errorf("got %d composite indexes, want 0", len(annots.Indexes))
	}
	if annots.Migration != MigrationPolicyUnrestricted {
		t.Errorf("Migration = %q, want unrestricted", annots.Migration)
	}

	if err := Validate(annots); err != nil {
		t.Fatalf("pre-existing vocabulary failed validation: %v", err)
	}
}
