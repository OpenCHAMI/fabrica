// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"go/ast"
	"go/parser"
	"go/token"
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
				for name, fieldAnnots := range annots.Fields {
					specFields[name] = fieldAnnots
				}
			}
		}
	}

	if result == nil {
		t.Fatalf("type %q not found in source", typeName)
	}

	for name, fieldAnnots := range specFields {
		result.Fields[name] = fieldAnnots
	}

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
			annotation: "// +fabrica:index:fields=value,owner",
			wantFields: []string{"value", "owner"},
			wantType:   IndexTypeBTree,
		},
		{
			name:       "named unique",
			annotation: "// +fabrica:index:fields=value,owner:name=idx_value_owner:unique",
			wantFields: []string{"value", "owner"},
			wantName:   "idx_value_owner",
			wantUnique: true,
			wantType:   IndexTypeBTree,
		},
		{
			name:       "explicit type",
			annotation: "// +fabrica:index:fields=value,owner:type=gin",
			wantFields: []string{"value", "owner"},
			wantType:   IndexTypeGIN,
		},
		{
			name:       "whitespace in field list is trimmed",
			annotation: "// +fabrica:index:fields=value, owner",
			wantFields: []string{"value", "owner"},
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
		{"bad type", "// +fabrica:index:fields=a,b:type=bogus", "unknown index type"},
		{"unknown param", "// +fabrica:index:fields=a,b:sorted", "unknown composite index parameter"},
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
				a.Indexes = []*CompositeIndex{{Fields: []string{"value"}, Type: IndexTypeBTree}}
				return a
			},
			wantErr: "use +fabrica:field:index",
		},
		{
			name: "duplicate column rejected",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{{Fields: []string{"value", "value"}, Type: IndexTypeBTree}}
				return a
			},
			wantErr: "more than once",
		},
		{
			name: "duplicate index name rejected",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{
					{Fields: []string{"value", "owner"}, Name: "idx_a", Type: IndexTypeBTree},
					{Fields: []string{"owner", "value"}, Name: "idx_a", Type: IndexTypeBTree},
				}
				return a
			},
			wantErr: "duplicate composite index name",
		},
		{
			name: "generic storage rejected",
			build: func() *ResourceAnnotations {
				a := NewResourceAnnotations()
				a.IsResource = true
				a.StorageMode = StorageModeGeneric
				a.Indexes = []*CompositeIndex{{Fields: []string{"value", "owner"}, Type: IndexTypeBTree}}
				return a
			},
			wantErr: "require +fabrica:storage=dedicated",
		},
		{
			name: "valid composite index accepted",
			build: func() *ResourceAnnotations {
				a := dedicatedResource()
				a.Indexes = []*CompositeIndex{{Fields: []string{"value", "owner"}, Type: IndexTypeBTree}}
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

// dedicatedResource returns a dedicated-mode resource with one annotated field,
// which satisfies validateDedicatedStorage.
func dedicatedResource() *ResourceAnnotations {
	a := NewResourceAnnotations()
	a.IsResource = true
	a.StorageMode = StorageModeDedicated
	a.Fields["Value"] = NewFieldAnnotations("Value")
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := dedicatedResource()
			a.Indexes = []*CompositeIndex{{Fields: []string{"value", "owner"}, Type: tt.indexType}}

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
