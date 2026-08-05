// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
)

const auditRecordSchema = `package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type AuditRecord struct {
	ent.Schema
}

func (AuditRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("uid").Unique(),
		field.String("action"),
	}
}
`

const auditCRUD = `package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"supplementaltest/internal/storage/ent"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file:audit?mode=memory&cache=shared&_fk=1")
	if err != nil {
		log.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	defer client.Close()

	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("MIGRATE FAILED: %v", err)
	}

	created, err := client.AuditRecord.Create().SetUID("audit-1").SetAction("login").Save(ctx)
	if err != nil {
		log.Fatalf("CREATE FAILED: %v", err)
	}
	got, err := client.AuditRecord.Get(ctx, created.ID)
	if err != nil {
		log.Fatalf("READ FAILED: %v", err)
	}
	if got.UID != "audit-1" || got.Action != "login" {
		log.Fatalf("unexpected audit record: %#v", got)
	}
	fmt.Println("supplemental schema round-trip ok")
}
`

func TestSupplementalEntSchemasSurviveRegeneration(t *testing.T) {
	const modulePath = "supplementaltest"

	dir := generateProject(t, modulePath, false)
	schemaPath := filepath.Join(dir, "internal", "storage", "ent", "schema", "audit_record.go")
	migrationPath := filepath.Join(dir, "migrations", "001_audit_record.sql")
	migration := "CREATE TABLE audit_records (uid text primary key, action text not null);\n"

	writeFile(t, schemaPath, auditRecordSchema)
	writeFile(t, migrationPath, migration)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir generated project: %v", err)
	}

	gen := NewGenerator(dir, "main", modulePath)
	gen.StorageType = "ent"
	gen.DBDriver = "postgres"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	if err := gen.RegisterResource(&testfixtures.Widget{}); err != nil {
		t.Fatalf("RegisterResource: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := gen.GenerateEntSchemas(); err != nil {
			t.Fatalf("GenerateEntSchemas pass %d: %v", i+1, err)
		}
	}

	schemaAfter, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read supplemental schema: %v", err)
	}
	if string(schemaAfter) != auditRecordSchema {
		t.Fatalf("supplemental schema was modified:\n%s", schemaAfter)
	}
	migrationAfter, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read supplemental migration: %v", err)
	}
	if string(migrationAfter) != migration {
		t.Fatalf("supplemental migration was modified:\n%s", migrationAfter)
	}
	if err := os.Chdir(origDir); err != nil {
		t.Fatalf("restore working directory before module setup: %v", err)
	}

	prepareModule(t, dir, modulePath, true)
	runEntCodegen(t, dir, modulePath)
	writeFile(t, filepath.Join(dir, "cmd", "auditcrud", "main.go"), auditCRUD)
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Skipf("go mod tidy failed (%v): %s", err, out)
	}
	out, err := runIn(filepath.Join(dir, "cmd", "auditcrud"), "go", "run", ".")
	if err != nil {
		t.Fatalf("supplemental schema CRUD failed:\n%s", out)
	}
	if !strings.Contains(out, "supplemental schema round-trip ok") {
		t.Fatalf("unexpected supplemental CRUD output:\n%s", out)
	}
}
