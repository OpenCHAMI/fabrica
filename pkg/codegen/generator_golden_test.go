// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/annotations"
)

// updateGolden rewrites the golden files instead of comparing against them.
//
//	go test ./pkg/codegen/ -run Golden -update
var updateGolden = flag.Bool("update", false, "rewrite golden files")

// copyrightYearRE normalizes the generated copyright line, which embeds
// time.Now().Year() and would otherwise break these tests every January.
var copyrightYearRE = regexp.MustCompile(`(?m)^// Copyright © \d{4} `)

func normalizeGolden(b []byte) []byte {
	return copyrightYearRE.ReplaceAll(b, []byte("// Copyright © <year> "))
}

// assertGolden compares got against testdata/golden/<name>, or rewrites it
// when -update is passed.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	got = normalizeGolden(got)
	path := filepath.Join("testdata", "golden", name)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run `go test ./pkg/codegen/ -run Golden -update` to create it): %v", path, err)
	}

	if string(got) != string(normalizeGolden(want)) {
		t.Errorf("generated output differs from %s\n--- want ---\n%s\n--- got ---\n%s",
			path, want, got)
	}
}

// generateDedicatedSchema runs the real generation path in a temp dir and
// returns the emitted dedicated Ent schema for the resource.
func generateDedicatedSchema(t *testing.T, resource interface{}, name string, annots *annotations.ResourceAnnotations) []byte {
	t.Helper()

	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	// Restore before returning: the caller resolves golden paths relative to
	// the package directory, not the temp dir.
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	gen.StorageType = "ent"
	gen.DBDriver = "postgres"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}
	if err := gen.RegisterResource(resource); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	gen.SetResourceAnnotations(name, annots)

	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}

	schemaFile := filepath.Join("internal", "storage", "ent", "schema", strings.ToLower(name)+".go")
	content, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read generated schema: %v", err)
	}

	return content
}

type goldenTokenSpec struct {
	Value     string `json:"value" validate:"required"`
	Owner     string `json:"owner"`
	Slug      string `json:"slug"`
	Note      string `json:"note"`
	UseCount  int    `json:"use_count"`
	Revoked   bool   `json:"revoked"`
	CreatedBy string `json:"created_by"`
}

type goldenToken struct {
	Spec goldenTokenSpec
}

// TestGoldenDedicatedSchemaFullVocabulary pins the emitted Ent schema for a
// resource that exercises every annotation the dedicated template understands,
// old and new.
func TestGoldenDedicatedSchemaFullVocabulary(t *testing.T) {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	annots.Migration = annotations.MigrationPolicyAdditiveOnly

	// Pre-existing vocabulary: hashed + sensitive + immutable.
	value := annotations.NewFieldAnnotations("Value")
	value.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmBcrypt, Cost: 12},
	}
	value.Sensitive = true
	value.Immutable = true
	annots.Fields["Value"] = value

	// New: size + explicit notnull.
	owner := annotations.NewFieldAnnotations("Owner")
	owner.Size = 253
	owner.NotNull = true
	annots.Fields["Owner"] = owner

	// New: named unique index on a single column.
	slug := annotations.NewFieldAnnotations("Slug")
	slug.Index = &annotations.IndexConfig{
		Type:   annotations.IndexTypeBTree,
		Unique: true,
		Name:   "idx_token_slug",
	}
	annots.Fields["Slug"] = slug

	// New: explicit nullable overriding the struct-tag inference.
	note := annotations.NewFieldAnnotations("Note")
	note.Nullable = true
	note.Size = 1024
	annots.Fields["Note"] = note

	// Pre-existing: default on an int.
	useCount := annotations.NewFieldAnnotations("UseCount")
	useCount.Default = "0"
	annots.Fields["UseCount"] = useCount

	// Pre-existing: default on a bool.
	revoked := annotations.NewFieldAnnotations("Revoked")
	revoked.Default = "false"
	annots.Fields["Revoked"] = revoked

	// New: relation. Parsed and validated in 01a; edge emission is deferred,
	// so this must NOT change the generated schema.
	createdBy := annotations.NewFieldAnnotations("CreatedBy")
	createdBy.Relation = &annotations.RelationConfig{
		Kind:     annotations.RelationBelongsTo,
		Target:   "User",
		OnDelete: annotations.OnDeleteCascade,
	}
	annots.Fields["CreatedBy"] = createdBy

	// New: composite indexes, one plain and one named-unique.
	annots.Indexes = []*annotations.CompositeIndex{
		{Fields: []string{"Owner", "Revoked"}, Type: annotations.IndexTypeBTree},
		{
			Fields: []string{"Owner", "Slug"},
			Name:   "idx_token_owner_slug",
			Unique: true,
			Type:   annotations.IndexTypeBTree,
		},
	}

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	got := generateDedicatedSchema(t, &goldenToken{}, "goldenToken", annots)
	assertGolden(t, "token_dedicated_full.go.golden", got)
}

// typedTokenSpec mirrors the field types on tokensmith's BootstrapTokenPolicy
// and RefreshTokenFamily, which is what surfaced the missing type mapping.
type typedTokenSpec struct {
	Subject        string            `json:"subject" validate:"required"`
	UsageCount     int               `json:"usage_count"`
	Revoked        bool              `json:"revoked"`
	SequenceNumber int64             `json:"sequence_number"`
	Weight         float64           `json:"weight"`
	TTL            time.Duration     `json:"ttl"`
	IssuedAt       time.Time         `json:"issued_at" validate:"required"`
	ConsumedAt     *time.Time        `json:"consumed_at"`
	Scopes         []string          `json:"scopes"`
	Fingerprint    []byte            `json:"fingerprint"`
	ReplayAttempts []time.Time       `json:"replay_attempts"`
	Labels         map[string]string `json:"labels"`
	Unmapped       []int             `json:"unmapped"`
}

type typedToken struct {
	Spec typedTokenSpec
}

// TestGoldenDedicatedSchemaTypeMapping pins the Ent field type chosen for each
// Go type. Before this mapping existed, every type below except string, int and
// bool fell through to field.String — silently wrong for ordering, range
// queries and indexes.
func TestGoldenDedicatedSchemaTypeMapping(t *testing.T) {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	// One annotated field proves annotations still compose with the new types.
	issuedAt := annotations.NewFieldAnnotations("IssuedAt")
	issuedAt.Immutable = true
	annots.Fields["IssuedAt"] = issuedAt

	scopes := annotations.NewFieldAnnotations("Scopes")
	scopes.Immutable = true
	annots.Fields["Scopes"] = scopes

	fingerprint := annotations.NewFieldAnnotations("Fingerprint")
	fingerprint.Sensitive = true
	annots.Fields["Fingerprint"] = fingerprint

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	got := generateDedicatedSchema(t, &typedToken{}, "typedToken", annots)
	assertGolden(t, "token_dedicated_types.go.golden", got)

	// Spot-check the mapping directly, so a wrong golden update is still caught.
	for _, want := range []string{
		`field.Int64("sequence_number")`,
		`field.Float("weight")`,
		`field.Int64("ttl")`,
		`field.Time("issued_at")`,
		`field.Time("consumed_at")`,
		`Nillable()`,
		`field.Strings("scopes")`,
		`field.Bytes("fingerprint")`,
		`field.JSON("replay_attempts", []time.Time{})`,
		`field.JSON("labels", map[string]string{})`,
		`UNMAPPED TYPE: []int`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("generated schema missing %q", want)
		}
	}

	// time.Duration must not silently become a string.
	if strings.Contains(string(got), `field.String("ttl")`) {
		t.Error("time.Duration was emitted as a string column")
	}
}

type baselineTokenSpec struct {
	Value string `json:"value" validate:"required"`
	Name  string `json:"name"`
}

type baselineToken struct {
	Spec baselineTokenSpec
}

// TestGoldenDedicatedSchemaBaseline pins the emitted schema for a resource that
// uses ONLY the pre-01a vocabulary. If extending the vocabulary changes this
// output, backward compatibility has been broken.
func TestGoldenDedicatedSchemaBaseline(t *testing.T) {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	value := annotations.NewFieldAnnotations("Value")
	value.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmBcrypt, Cost: 12},
	}
	value.Sensitive = true
	value.Immutable = true
	annots.Fields["Value"] = value

	name := annotations.NewFieldAnnotations("Name")
	name.Index = &annotations.IndexConfig{Type: annotations.IndexTypeBTree}
	name.Unique = true
	annots.Fields["Name"] = name

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	got := generateDedicatedSchema(t, &baselineToken{}, "baselineToken", annots)
	assertGolden(t, "token_dedicated_baseline.go.golden", got)
}
