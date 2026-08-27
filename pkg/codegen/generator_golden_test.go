// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"flag"
	"os"
	"os/exec"
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

	content, err := generateDedicatedSchemaContent(t, resource, name, annots)
	if err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}

	return content
}

func generateDedicatedSchemaError(t *testing.T, resource interface{}, name string, annots *annotations.ResourceAnnotations) error {
	t.Helper()

	_, err := generateDedicatedSchemaContent(t, resource, name, annots)
	return err
}

func generateDedicatedSchemaContent(t *testing.T, resource interface{}, name string, annots *annotations.ResourceAnnotations) ([]byte, error) {
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
		return nil, err
	}

	schemaFile := filepath.Join("internal", "storage", "ent", "schema", strings.ToLower(name)+".go")
	content, err := os.ReadFile(schemaFile)
	if err != nil {
		return nil, err
	}

	return content, nil
}

type GoldenTokenSpec struct {
	Value      string `json:"value" validate:"required"`
	Checksum   string `json:"checksum"`
	Owner      string `json:"owner"`
	Slug       string `json:"slug"`
	Note       string `json:"note" validate:"required"`
	SearchText string `json:"search_text"`
	UseCount   int    `json:"use_count"`
	Revoked    bool   `json:"revoked"`
	CreatedBy  string `json:"created_by"`
}

type GoldenToken struct {
	Spec GoldenTokenSpec
}

func fullVocabularyAnnotations() *annotations.ResourceAnnotations {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	annots.Migration = annotations.MigrationPolicyAdditiveOnly

	value := annotations.NewFieldAnnotations("Value")
	value.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmBcrypt, Cost: 12},
	}
	value.Sensitive = true
	value.Immutable = true
	annots.Fields["Value"] = value

	checksum := annotations.NewFieldAnnotations("Checksum")
	checksum.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmSHA256},
	}
	checksum.Nullable = true
	annots.Fields["Checksum"] = checksum

	owner := annotations.NewFieldAnnotations("Owner")
	owner.Size = 253
	owner.NotNull = true
	annots.Fields["Owner"] = owner

	slug := annotations.NewFieldAnnotations("Slug")
	slug.Index = &annotations.IndexConfig{
		Type:   annotations.IndexTypeBTree,
		Unique: true,
		Name:   "idx_token_slug",
	}
	annots.Fields["Slug"] = slug

	note := annotations.NewFieldAnnotations("Note")
	note.Nullable = true
	note.Size = 1024
	annots.Fields["Note"] = note

	searchText := annotations.NewFieldAnnotations("SearchText")
	searchText.Index = &annotations.IndexConfig{Type: annotations.IndexTypeGIN}
	annots.Fields["SearchText"] = searchText

	useCount := annotations.NewFieldAnnotations("UseCount")
	useCount.Default = "0"
	annots.Fields["UseCount"] = useCount

	revoked := annotations.NewFieldAnnotations("Revoked")
	revoked.Default = "false"
	annots.Fields["Revoked"] = revoked

	createdBy := annotations.NewFieldAnnotations("CreatedBy")
	createdBy.Relation = &annotations.RelationConfig{
		Kind:     annotations.RelationBelongsTo,
		Target:   "User",
		OnDelete: annotations.OnDeleteCascade,
	}
	annots.Fields["CreatedBy"] = createdBy

	annots.Indexes = []*annotations.CompositeIndex{
		{Fields: []string{"Owner", "Revoked"}, Type: annotations.IndexTypeBTree},
		{
			Fields: []string{"Owner", "Slug"},
			Name:   "idx_token_owner_slug",
			Unique: true,
			Type:   annotations.IndexTypeBTree,
		},
		{Fields: []string{"Owner", "SearchText"}, Type: annotations.IndexTypeGIN},
	}

	return annots
}

// TestGoldenDedicatedSchemaFullVocabulary pins the emitted Ent schema for a
// resource that exercises every annotation the dedicated template understands,
// old and new.
func TestGoldenDedicatedSchemaFullVocabulary(t *testing.T) {
	annots := fullVocabularyAnnotations()

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	got := generateDedicatedSchema(t, &GoldenToken{}, "GoldenToken", annots)
	assertGolden(t, "token_dedicated_full.go.golden", got)
}

type TypedTokenSpec struct {
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
}

type TypedToken struct {
	Spec TypedTokenSpec
}

func typeMappingAnnotations() *annotations.ResourceAnnotations {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	issuedAt := annotations.NewFieldAnnotations("IssuedAt")
	issuedAt.Immutable = true
	annots.Fields["IssuedAt"] = issuedAt

	scopes := annotations.NewFieldAnnotations("Scopes")
	scopes.Immutable = true
	annots.Fields["Scopes"] = scopes

	fingerprint := annotations.NewFieldAnnotations("Fingerprint")
	fingerprint.Sensitive = true
	annots.Fields["Fingerprint"] = fingerprint

	return annots
}

func TestGoldenDedicatedSchemaTypeMapping(t *testing.T) {
	annots := typeMappingAnnotations()

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	got := generateDedicatedSchema(t, &TypedToken{}, "TypedToken", annots)
	assertGolden(t, "token_dedicated_types.go.golden", got)

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
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("generated schema missing %q", want)
		}
	}

	if strings.Contains(string(got), `field.String("ttl")`) {
		t.Error("time.Duration was emitted as a string column")
	}
}

func TestGenerateDedicatedSchemaRejectsUnsupportedType(t *testing.T) {
	type UnsupportedTokenSpec struct {
		Unmapped []int `json:"unmapped"`
	}

	type UnsupportedToken struct {
		Spec UnsupportedTokenSpec
	}

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	annots.Fields["Unmapped"] = annotations.NewFieldAnnotations("Unmapped")

	err := generateDedicatedSchemaError(t, &UnsupportedToken{}, "UnsupportedToken", annots)
	if err == nil || !strings.Contains(err.Error(), `unsupported dedicated Ent field type "[]int"`) {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}

type BaselineTokenSpec struct {
	Value       string `json:"value" validate:"required"`
	DisplayName string `json:"display_name"`
}

type BaselineToken struct {
	Spec BaselineTokenSpec
}

type PlainTokenSpec struct {
	DisplayName string `json:"display_name"`
}

type PlainToken struct {
	Spec PlainTokenSpec
}

func baselineAnnotations() *annotations.ResourceAnnotations {
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

	displayName := annotations.NewFieldAnnotations("DisplayName")
	displayName.Index = &annotations.IndexConfig{Type: annotations.IndexTypeBTree}
	displayName.Unique = true
	annots.Fields["DisplayName"] = displayName

	return annots
}

func plainAnnotations() *annotations.ResourceAnnotations {
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	annots.Fields["DisplayName"] = annotations.NewFieldAnnotations("DisplayName")
	return annots
}

// TestGoldenDedicatedSchemaBaseline pins the emitted schema for a resource that
// uses ONLY the pre-01a vocabulary. If extending the vocabulary changes this
// output, backward compatibility has been broken.
func TestGoldenDedicatedSchemaBaseline(t *testing.T) {
	annots := baselineAnnotations()

	if err := annotations.Validate(annots); err != nil {
		t.Fatalf("fixture failed validation: %v", err)
	}

	got := generateDedicatedSchema(t, &BaselineToken{}, "BaselineToken", annots)
	assertGolden(t, "token_dedicated_baseline.go.golden", got)
}

func TestGeneratedDedicatedEntSchemasCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated Ent compile test in short mode")
	}

	schemas := map[string][]byte{
		"goldentoken.go":   generateDedicatedSchema(t, &GoldenToken{}, "GoldenToken", fullVocabularyAnnotations()),
		"typedtoken.go":    generateDedicatedSchema(t, &TypedToken{}, "TypedToken", typeMappingAnnotations()),
		"baselinetoken.go": generateDedicatedSchema(t, &BaselineToken{}, "BaselineToken", baselineAnnotations()),
		"plaintoken.go":    generateDedicatedSchema(t, &PlainToken{}, "PlainToken", plainAnnotations()),
	}

	tmpDir := t.TempDir()
	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}
	for name, content := range schemas {
		if err := os.WriteFile(filepath.Join(schemaDir, name), content, 0o644); err != nil {
			t.Fatalf("write generated schema %s: %v", name, err)
		}
	}

	goMod := []byte("module generatedschema\n\ngo 1.26.6\n\nrequire (\n\tentgo.io/ent v0.14.5\n\tgolang.org/x/crypto v0.43.0\n)\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = tmpDir
	output, err := tidy.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dedicated Ent schema module failed go mod tidy: %v\n%s", err, output)
	}

	cmd := exec.Command("go", "test", "./schema")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dedicated Ent schemas failed to compile: %v\n%s", err, output)
	}

	entGenerate := exec.Command("go", "run", "-mod=mod", "entgo.io/ent/cmd/ent", "generate", "./schema")
	entGenerate.Dir = tmpDir
	output, err = entGenerate.CombinedOutput()
	if err != nil {
		t.Fatalf("generated dedicated Ent schemas failed Ent codegen: %v\n%s", err, output)
	}

	tidy = exec.Command("go", "mod", "tidy")
	tidy.Dir = tmpDir
	output, err = tidy.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Ent module failed post-generation go mod tidy: %v\n%s", err, output)
	}

	moduleTest := exec.Command("go", "test", "./...")
	moduleTest.Dir = tmpDir
	output, err = moduleTest.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Ent module failed to compile: %v\n%s", err, output)
	}
}

func TestGenerateEntSchemasRejectsUnsupportedStorageTransforms(t *testing.T) {
	for _, tt := range []struct {
		name      string
		configure func(*annotations.FieldAnnotations)
		wantErr   string
	}{
		{
			name: "encrypted storage",
			configure: func(field *annotations.FieldAnnotations) {
				field.Storage = &annotations.StorageConfig{
					Type:       annotations.StorageTypeEncrypted,
					Encryption: &annotations.EncryptionConfig{Algorithm: "aes256", KeySource: "env"},
				}
			},
			wantErr: "encrypted storage is not emitted yet",
		},
		{
			name: "argon2 hashing",
			configure: func(field *annotations.FieldAnnotations) {
				field.Storage = &annotations.StorageConfig{
					Type: annotations.StorageTypeHashed,
					Hash: &annotations.HashConfig{Algorithm: annotations.HashAlgorithmArgon2, Cost: 19456},
				}
			},
			wantErr: "argon2 hashing is not emitted yet",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			annots := annotations.NewResourceAnnotations()
			annots.IsResource = true
			annots.StorageMode = annotations.StorageModeDedicated
			field := annotations.NewFieldAnnotations("Value")
			tt.configure(field)
			annots.Fields["Value"] = field

			err := generateDedicatedSchemaError(t, &GoldenToken{}, "GoldenToken", annots)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
