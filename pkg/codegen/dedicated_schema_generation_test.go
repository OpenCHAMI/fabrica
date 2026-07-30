// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/annotations"
)

type DedicatedShapeSpec struct {
	Name        string
	SpecName    string
	Status      string
	Metadata    string
	Plain       string
	Enabled     bool
	Count       int
	Sequence    int64
	Ratio       float64
	ObservedAt  time.Time
	Aliases     []string
	Description *string
	Ready       *bool
	Limit       *int
	Offset      *int64
	Threshold   *float64
	ExpiresAt   *time.Time
	Secret      string
	Code        string
}

type DedicatedShape struct {
	Spec DedicatedShapeSpec
}

type generatedDedicatedSchema struct {
	root    string
	content string
}

func TestGeneratedDedicatedSchema_renders_resolved_field_shapes_and_modifiers(t *testing.T) {
	// Given
	schema := generateDedicatedShapeSchema(t, "postgres").content

	// When
	wantFragments := []string{
		`field.String("spec_name")`,
		`field.String("spec_spec_name")`,
		`field.String("spec_status")`,
		`field.String("spec_metadata")`,
		`field.String("spec_plain")`,
		`field.Bool("spec_enabled").`, `Default(false).`,
		`field.Int("spec_count").`, `Default(0).`,
		`field.Int64("spec_sequence").`, `Default(-9).`,
		`field.Float("spec_ratio").`, `Default(2.0).`,
		`field.Time("spec_observed_at")`,
		`field.Strings("spec_aliases")`,
		`field.String("spec_description").`, `field.Bool("spec_ready").`,
		`field.Int("spec_limit").`, `field.Int64("spec_offset").`,
		`field.Float("spec_threshold").`, `field.Time("spec_expires_at").`,
		`Sensitive().`, `Immutable().`, `Unique().`,
	}

	// Then
	for _, fragment := range wantFragments {
		if !strings.Contains(schema, fragment) {
			t.Errorf("generated schema missing %q\n%s", fragment, schema)
		}
	}
	if got := strings.Count(schema, "Nillable()."); got != 6 {
		t.Errorf("Nillable() count = %d, want 6\n%s", got, schema)
	}
	if strings.Contains(schema, `index.Fields("spec_code")`) {
		t.Errorf("unique B-tree field has a redundant standalone index\n%s", schema)
	}
}

func TestGeneratedDedicatedSchema_keeps_bcrypt_as_string_storage_without_crypto_hooks(t *testing.T) {
	// Given
	schema := generateDedicatedShapeSchema(t, "sqlite").content

	// When
	forbidden := []string{"bcrypt", "entgo.io/ent/dialect", "func (DedicatedShape) Hooks", "sha256", "SchemaType("}

	// Then
	for _, fragment := range forbidden {
		if strings.Contains(schema, fragment) {
			t.Errorf("generated schema contains forbidden crypto/dialect fragment %q\n%s", fragment, schema)
		}
	}
	if !strings.Contains(schema, `field.String("spec_secret").`) {
		t.Fatalf("bcrypt-tagged field is not rendered as string storage\n%s", schema)
	}
}

func TestDedicatedSchemaData_rejects_malformed_typed_state_before_rendering(t *testing.T) {
	// Given
	resolved := &annotations.ResolvedResourceStorage{
		Name:    "Broken",
		Storage: annotations.ResourceStorageDedicated,
		Dialect: annotations.DialectPostgreSQL,
		Fields: []annotations.ResolvedFieldStorage{{
			GoName:      "Mystery",
			JSONName:    "mystery",
			Optionality: annotations.OptionalityRequired,
			Transform:   annotations.StorageTransform{Kind: annotations.TransformStandard},
			Index:       annotations.IndexNone,
		}},
	}

	// When
	_, err := buildDedicatedSchemaData(resolved, 2026)

	// Then
	if err == nil || !strings.Contains(err.Error(), "Mystery") {
		t.Fatalf("malformed typed state error = %v, want field-located rejection", err)
	}
}

func generateDedicatedShapeSchema(t *testing.T, driver string) generatedDedicatedSchema {
	t.Helper()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "dedicated_shape_types.go")
	if err := os.WriteFile(sourcePath, []byte(dedicatedShapeSource), 0o644); err != nil {
		t.Fatalf("write dedicated schema source: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change to fixture directory: %v", err)
	}

	gen := NewGenerator(root, "fixture", "example.com/fixture")
	gen.StorageType = "ent"
	gen.DBDriver = driver
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	if err := gen.RegisterResourceFromSource(&DedicatedShape{}, sourcePath); err != nil {
		t.Fatalf("register resource source: %v", err)
	}
	if err := gen.PrepareResourceAnnotations(); err != nil {
		t.Fatalf("prepare resource annotations: %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("generate Ent schemas: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, "internal", "storage", "ent", "schema", "dedicatedshape.go"))
	if err != nil {
		t.Fatalf("read dedicated schema: %v", err)
	}
	return generatedDedicatedSchema{root: root, content: string(content)}
}

const dedicatedShapeSource = `package fixture

import "time"

// +fabrica:resource
// +fabrica:storage=dedicated
type DedicatedShape struct { Spec DedicatedShapeSpec }

type DedicatedShapeSpec struct {
	Name string ` + "`json:\"name\" validate:\"required\"`" + `
	SpecName string ` + "`json:\"spec_name\" validate:\"required\"`" + `
	Status string ` + "`json:\"status\" validate:\"required\"`" + `
	Metadata string ` + "`json:\"metadata\" validate:\"required\"`" + `
	Plain string ` + "`json:\"plain\" validate:\"required\"`" + `
	// +fabrica:field:default=false
	Enabled bool ` + "`json:\"enabled\"`" + `
	// +fabrica:field:default=0
	Count int ` + "`json:\"count\"`" + `
	// +fabrica:field:default=-9
	Sequence int64 ` + "`json:\"sequence\"`" + `
	// +fabrica:field:default=2
	Ratio float64 ` + "`json:\"ratio\"`" + `
	ObservedAt time.Time ` + "`json:\"observed_at\" validate:\"required\"`" + `
	Aliases []string ` + "`json:\"aliases\"`" + `
	// +fabrica:field:default=fallback
	Description *string ` + "`json:\"description\"`" + `
	Ready *bool ` + "`json:\"ready\"`" + `
	Limit *int ` + "`json:\"limit\"`" + `
	Offset *int64 ` + "`json:\"offset\"`" + `
	Threshold *float64 ` + "`json:\"threshold\"`" + `
	ExpiresAt *time.Time ` + "`json:\"expires_at\"`" + `
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Secret string ` + "`json:\"secret\" validate:\"required\"`" + `
	// +fabrica:field:index
	// +fabrica:field:unique
	Code string ` + "`json:\"code\"`" + `
}
`
