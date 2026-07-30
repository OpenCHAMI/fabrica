// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type DedicatedIndexSpec struct {
	Lookup      string
	Slug        string
	Tags        []string
	Fingerprint *string
}

type DedicatedIndex struct {
	Spec DedicatedIndexSpec
}

func TestGeneratedDedicatedIndex_baseline_portable_btree_and_unique(t *testing.T) {
	// Given
	schema := generateDedicatedIndexSchema(t, "postgres", portableDedicatedIndexSource)

	// When
	lookupIndex := `index.Fields("spec_lookup")`
	slugField := `field.String("spec_slug").`

	// Then
	if !strings.Contains(schema, lookupIndex) {
		t.Fatalf("portable B-tree index is absent\n%s", schema)
	}
	if strings.Contains(schema, "entsql.") || strings.Contains(schema, "entgo.io/ent/dialect") {
		t.Fatalf("portable B-tree index has unnecessary dialect annotation\n%s", schema)
	}
	if !strings.Contains(schema, slugField) || !strings.Contains(schema[strings.Index(schema, slugField):], "Unique().") {
		t.Fatalf("unique field modifier is absent\n%s", schema)
	}
	if strings.Contains(schema, `index.Fields("spec_slug")`) {
		t.Fatalf("unique-only field has a redundant standalone index\n%s", schema)
	}
}

func TestGeneratedDedicatedIndex_legacy_map_annotation_does_not_compile(t *testing.T) {
	// Given
	root := t.TempDir()
	writeDedicatedIndexFixtureFile(t, filepath.Join(root, "go.mod"), "module example.com/invalid-index\n\ngo 1.24.0\n\nrequire entgo.io/ent v0.14.5\n")
	writeDedicatedIndexFixtureFile(t, filepath.Join(root, "invalid_test.go"), legacyMapAnnotationSource)
	runDedicatedSchemaCommand(t, root, "go", "mod", "tidy")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "go", "test", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Stdout = &output
	cmd.Stderr = &output

	// When
	err := cmd.Run()

	// Then
	if err == nil {
		t.Fatalf("legacy map annotation compiled; expected schema.Annotation interface failure\n%s", output.String())
	}
	if ctx.Err() != nil {
		t.Fatalf("legacy map annotation compile probe timed out: %v", ctx.Err())
	}
	if !strings.Contains(output.String(), "does not implement schema.Annotation") {
		t.Fatalf("compile failure does not prove the annotation contract\n%s", output.String())
	}
}

func TestGeneratedDedicatedIndex_postgresql_methods_use_ent_annotations(t *testing.T) {
	// Given
	schema := generateDedicatedIndexSchema(t, "postgres", postgresqlDedicatedIndexSource)

	// When
	wantFragments := []string{
		`"entgo.io/ent/dialect/entsql"`,
		`index.Fields("spec_tags")`,
		`entsql.IndexType("GIN")`,
		`field.String("spec_fingerprint")`,
		`index.Fields("spec_fingerprint")`,
		`entsql.IndexType("HASH")`,
	}

	// Then
	for _, fragment := range wantFragments {
		if !strings.Contains(schema, fragment) {
			t.Errorf("PostgreSQL schema missing %q\n%s", fragment, schema)
		}
	}
	if strings.Contains(schema, "map[string]interface{}") {
		t.Fatalf("PostgreSQL schema contains legacy map annotation\n%s", schema)
	}
	if strings.Contains(schema, `index.Fields("spec_slug")`) {
		t.Fatalf("unique B-tree field has a redundant standalone index\n%s", schema)
	}
	fingerprintField := strings.Index(schema, `field.String("spec_fingerprint")`)
	slugField := strings.Index(schema, `field.String("spec_slug")`)
	if fingerprintField < 0 || slugField <= fingerprintField || !strings.Contains(schema[fingerprintField:slugField], "Unique().") {
		t.Fatalf("unique hash field lost its uniqueness contract\n%s", schema)
	}
}

func generateDedicatedIndexSchema(t *testing.T, driver, source string) string {
	t.Helper()

	gen, root := newDedicatedIndexGenerator(t, driver, source)
	if err := gen.PrepareResourceAnnotations(); err != nil {
		t.Fatalf("prepare resource annotations: %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("generate Ent schemas: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "internal", "storage", "ent", "schema", "dedicatedindex.go"))
	if err != nil {
		t.Fatalf("read dedicated index schema: %v", err)
	}
	return string(content)
}

func newDedicatedIndexGenerator(t *testing.T, driver, source string) (*Generator, string) {
	t.Helper()

	root := t.TempDir()
	sourcePath := filepath.Join(root, "dedicated_index_types.go")
	writeDedicatedIndexFixtureFile(t, sourcePath, source)
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
	if err := gen.RegisterResourceFromSource(&DedicatedIndex{}, sourcePath); err != nil {
		t.Fatalf("register resource source: %v", err)
	}
	return gen, root
}

func writeDedicatedIndexFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", filepath.Base(path), err)
	}
}

const portableDedicatedIndexSource = `package fixture

import "time"

// +fabrica:resource
// +fabrica:storage=dedicated
type DedicatedIndex struct { Spec DedicatedIndexSpec }
type DedicatedIndexSpec struct {
	// +fabrica:field:index=btree
	Lookup string ` + "`json:\"lookup\" validate:\"required\"`" + `
	// +fabrica:field:index=btree
	Enabled bool ` + "`json:\"enabled\"`" + `
	// +fabrica:field:index=btree
	Count int ` + "`json:\"count\"`" + `
	// +fabrica:field:index=btree
	Sequence int64 ` + "`json:\"sequence\"`" + `
	// +fabrica:field:index=btree
	Ratio float64 ` + "`json:\"ratio\"`" + `
	// +fabrica:field:index=btree
	ObservedAt time.Time ` + "`json:\"observed_at\"`" + `
	// +fabrica:field:index=btree
	Aliases []string ` + "`json:\"aliases\"`" + `
	// +fabrica:field:index=btree
	Optional *string ` + "`json:\"optional\"`" + `
	// +fabrica:field:unique
	Slug string ` + "`json:\"slug\" validate:\"required\"`" + `
}
`

const postgresqlDedicatedIndexSource = `package fixture

// +fabrica:resource
// +fabrica:storage=dedicated
type DedicatedIndex struct { Spec DedicatedIndexSpec }
type DedicatedIndexSpec struct {
	// +fabrica:field:index=gin
	Tags []string ` + "`json:\"tags\"`" + `
	// +fabrica:field:index=hash
	// +fabrica:field:unique
	Fingerprint *string ` + "`json:\"fingerprint\"`" + `
	// +fabrica:field:index=btree
	// +fabrica:field:unique
	Slug string ` + "`json:\"slug\" validate:\"required\"`" + `
}
`

const legacyMapAnnotationSource = `package invalidindex

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/index"
)

func indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tags").Annotations(map[string]interface{}{"gin": true}),
	}
}
`
