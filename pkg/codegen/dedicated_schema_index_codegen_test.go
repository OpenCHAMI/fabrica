//go:build integration

// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"path/filepath"
	"testing"
)

func TestGeneratedDedicatedIndex_codegen_build_descriptors_and_sqlite_ddl_by_dialect(t *testing.T) {
	tests := []struct {
		name           string
		driver         string
		source         string
		descriptorTest string
		migrationTest  string
	}{
		{
			name:           "PostgreSQL GIN and hash",
			driver:         "postgres",
			source:         postgresqlDedicatedIndexSource,
			descriptorTest: postgresqlIndexDescriptorTest,
		},
		{
			name:           "SQLite portable B-tree",
			driver:         "sqlite",
			source:         portableDedicatedIndexSource,
			descriptorTest: sqliteIndexDescriptorTest,
			migrationTest:  sqliteIndexMigrationTest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			gen, root := newDedicatedIndexGenerator(t, test.driver, test.source)
			if err := gen.PrepareResourceAnnotations(); err != nil {
				t.Fatalf("prepare annotations: %v", err)
			}
			if err := gen.GenerateEntSchemas(); err != nil {
				t.Fatalf("generate schemas: %v", err)
			}
			writeDedicatedIndexFixtureFile(t, filepath.Join(root, "go.mod"), dedicatedIndexFixtureGoMod)
			writeDedicatedIndexFixtureFile(
				t,
				filepath.Join(root, "internal", "storage", "ent", "schema", "dedicatedindex_descriptor_test.go"),
				test.descriptorTest,
			)
			runDedicatedSchemaCommand(t, root, "go", "mod", "tidy")

			// When
			runDedicatedSchemaCommand(t, root, "go", "run", "entgo.io/ent/cmd/ent@v0.14.5", "generate", "./internal/storage/ent/schema")
			if test.migrationTest != "" {
				writeDedicatedIndexFixtureFile(
					t,
					filepath.Join(root, "internal", "storage", "ent", "dedicatedindex_migration_test.go"),
					test.migrationTest,
				)
			}
			runDedicatedSchemaCommand(t, root, "go", "mod", "tidy")

			// Then
			runDedicatedSchemaCommand(t, root, "go", "test", "./internal/storage/ent/schema", "./internal/storage/ent")
			runDedicatedSchemaCommand(t, root, "go", "build", "./internal/storage/ent/...")
		})
	}
}

const dedicatedIndexFixtureGoMod = `module example.com/dedicated-index-fixture

go 1.24.0

require entgo.io/ent v0.14.5
`

const postgresqlIndexDescriptorTest = `package schema

import (
	"testing"

	"entgo.io/ent/dialect/entsql"
)

func TestDedicatedIndexDescriptors(t *testing.T) {
	want := map[string]string{"spec_tags": "GIN", "spec_fingerprint": "HASH"}
	for _, idx := range (DedicatedIndex{}).Indexes() {
		desc := idx.Descriptor()
		if len(desc.Fields) != 1 || want[desc.Fields[0]] == "" {
			continue
		}
		if len(desc.Annotations) != 1 {
			t.Fatalf("index %s annotations = %#v", desc.Fields[0], desc.Annotations)
		}
		annotation, ok := desc.Annotations[0].(*entsql.IndexAnnotation)
		if !ok || annotation.Type != want[desc.Fields[0]] {
			t.Fatalf("index %s annotation = %#v, want %s", desc.Fields[0], annotation, want[desc.Fields[0]])
		}
		delete(want, desc.Fields[0])
	}
	if len(want) != 0 {
		t.Fatalf("missing index descriptors: %#v", want)
	}
}
`

const sqliteIndexDescriptorTest = `package schema

import "testing"

func TestDedicatedIndexDescriptors(t *testing.T) {
	for _, idx := range (DedicatedIndex{}).Indexes() {
		desc := idx.Descriptor()
		if len(desc.Fields) == 1 && desc.Fields[0] == "spec_lookup" {
			if len(desc.Annotations) != 0 {
				t.Fatalf("portable index annotations = %#v", desc.Annotations)
			}
			return
		}
	}
	t.Fatal("spec_lookup descriptor is absent")
}
`

const sqliteIndexMigrationTest = `package ent

import (
	"database/sql"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"

	"example.com/dedicated-index-fixture/internal/storage/ent/dedicatedindex"
)

func TestDedicatedIndexSQLiteMigrationDDL(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:dedicated-index?mode=memory&cache=shared&_fk=1")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	client := NewClient(Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	defer client.Close()
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	rows, err := db.QueryContext(t.Context(), "SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ?", dedicatedindex.Table)
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	var ddl []string
	for rows.Next() {
		var statement sql.NullString
		if err := rows.Scan(&statement); err != nil { t.Fatal(err) }
		if statement.Valid { ddl = append(ddl, statement.String) }
	}
	if err := rows.Err(); err != nil { t.Fatal(err) }
	if !strings.Contains(strings.Join(ddl, "\n"), "spec_lookup") {
		t.Fatalf("SQLite migration DDL missing spec_lookup index: %v", ddl)
	}
}
`
