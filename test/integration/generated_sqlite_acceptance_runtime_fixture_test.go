// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const generatedSQLiteRuntimeTest = `package storage

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"github.com/openchami/fabrica/pkg/fabrica"
)

type sqliteColumn struct { Name, Type string; NotNull int; Default sql.NullString }

func openGeneratedSQLite(t *testing.T, path string) (*ent.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	SetEntClient(client)
	return client, db
}

func sqliteColumns(t *testing.T, db *sql.DB) map[string]sqliteColumn {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "PRAGMA table_info(tokens)")
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	columns := make(map[string]sqliteColumn)
	for rows.Next() {
		var sequence, primaryKey int
		var column sqliteColumn
		if err := rows.Scan(&sequence, &column.Name, &column.Type, &column.NotNull, &column.Default, &primaryKey); err != nil { t.Fatal(err) }
		columns[column.Name] = column
	}
	if err := rows.Err(); err != nil { t.Fatal(err) }
	return columns
}

func acceptanceToken(uid, slug string) *v1.Token {
	now := time.Date(2026, 7, 14, 1, 2, 3, 0, time.UTC)
	note := "optional"
	return &v1.Token{
		APIVersion: "acceptance.example.io/v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: uid, UID: uid, Namespace: "tenant-a", ResourceVersion: "7", CreatedAt: now, UpdatedAt: now, Labels: map[string]string{"suite": "sqlite"}, Annotations: map[string]string{"owner": "qa"}},
		Spec: v1.TokenSpec{Lookup: "lookup-"+uid, Slug: slug, ImmutableCode: "fixed", OptionalNote: &note},
		Status: v1.TokenStatus{State: "ready"},
	}
}

func TestGeneratedSQLite_schema_DDL(t *testing.T) {
	client, db := openGeneratedSQLite(t, filepath.Join(t.TempDir(), "schema.db"))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	columns := sqliteColumns(t, db)
	names := make([]string, 0, len(columns))
	for name := range columns { names = append(names, name) }
	sort.Strings(names)
	wantNames := []string{"annotations", "api_version", "created_at", "id", "kind", "labels", "name", "namespace", "resource_version", "spec_enabled", "spec_immutable_code", "spec_lookup", "spec_optional_note", "spec_retries", "spec_slug", "status", "uid", "updated_at"}
	if !reflect.DeepEqual(names, wantNames) { t.Fatalf("columns=%v want=%v", names, wantNames) }
	want := map[string]sqliteColumn{
		"id": {Name: "id", Type: "INTEGER", NotNull: 1},
		"uid": {Name: "uid", Type: "TEXT", NotNull: 1},
		"name": {Name: "name", Type: "TEXT", NotNull: 1},
		"namespace": {Name: "namespace", Type: "TEXT"},
		"api_version": {Name: "api_version", Type: "TEXT", NotNull: 1, Default: sql.NullString{String: "'v1'", Valid: true}},
		"kind": {Name: "kind", Type: "TEXT", NotNull: 1, Default: sql.NullString{String: "'Token'", Valid: true}},
		"created_at": {Name: "created_at", Type: "datetime", NotNull: 1},
		"updated_at": {Name: "updated_at", Type: "datetime", NotNull: 1},
		"resource_version": {Name: "resource_version", Type: "TEXT", NotNull: 1, Default: sql.NullString{String: "'1'", Valid: true}},
		"status": {Name: "status", Type: "json"},
		"labels": {Name: "labels", Type: "json"},
		"annotations": {Name: "annotations", Type: "json"},
		"spec_lookup": {Name: "spec_lookup", Type: "TEXT", NotNull: 1},
		"spec_slug": {Name: "spec_slug", Type: "TEXT", NotNull: 1},
		"spec_enabled": {Name: "spec_enabled", Type: "bool", NotNull: 1, Default: sql.NullString{String: "false", Valid: true}},
		"spec_retries": {Name: "spec_retries", Type: "INTEGER", NotNull: 1, Default: sql.NullString{String: "0", Valid: true}},
		"spec_immutable_code": {Name: "spec_immutable_code", Type: "TEXT", NotNull: 1},
		"spec_optional_note": {Name: "spec_optional_note", Type: "TEXT", Default: sql.NullString{String: "'fallback'", Valid: true}},
	}
	for name, expected := range want {
		if columns[name] != expected { t.Errorf("column %s=%#v want=%#v", name, columns[name], expected) }
	}
	var ddl string
	if err := db.QueryRowContext(t.Context(), "SELECT sql FROM sqlite_master WHERE type='table' AND name='tokens'").Scan(&ddl); err != nil { t.Fatal(err) }
	rows, err := db.QueryContext(t.Context(), "SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='tokens' ORDER BY name")
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	var indexes []string
	for rows.Next() { var statement sql.NullString; if err := rows.Scan(&statement); err != nil { t.Fatal(err) }; if statement.Valid { indexes = append(indexes, statement.String) } }
	if err := rows.Err(); err != nil { t.Fatal(err) }
	joined := strings.Join(indexes, "\n")
	if !strings.Contains(joined, "spec_lookup") || !strings.Contains(joined, "spec_slug") || !strings.Contains(joined, "UNIQUE") { t.Fatalf("indexes=%s", joined) }
	t.Logf("schema DDL table=%s indexes=%s", ddl, joined)
}

func TestGeneratedSQLite_raw_defaults_reject_scalar_NULL(t *testing.T) {
	client, db := openGeneratedSQLite(t, filepath.Join(t.TempDir(), "raw-defaults.db"))
	t.Cleanup(func() { _ = client.Close() })
	now := time.Date(2026, 7, 14, 1, 2, 3, 0, time.UTC)
	insert := "INSERT INTO tokens (uid,name,created_at,updated_at,spec_lookup,spec_slug,spec_immutable_code) VALUES (?,?,?,?,?,?,?)"
	if _, err := db.ExecContext(t.Context(), insert, "raw-default", "raw-default", now, now, "lookup", "raw-default", "fixed"); err != nil { t.Fatal(err) }
	var enabled bool
	var retries int
	var optional sql.NullString
	if err := db.QueryRowContext(t.Context(), "SELECT spec_enabled,spec_retries,spec_optional_note FROM tokens WHERE uid=?", "raw-default").Scan(&enabled, &retries, &optional); err != nil { t.Fatal(err) }
	if enabled || retries != 0 || !optional.Valid || optional.String != "fallback" { t.Fatalf("raw defaults enabled=%v retries=%d optional=%#v", enabled, retries, optional) }
	nullScalar := "INSERT INTO tokens (uid,name,created_at,updated_at,spec_lookup,spec_slug,spec_immutable_code,spec_enabled) VALUES (?,?,?,?,?,?,?,NULL)"
	if _, err := db.ExecContext(t.Context(), nullScalar, "raw-null", "raw-null", now, now, "lookup-null", "raw-null", "fixed"); err == nil { t.Fatal("explicit NULL scalar default succeeded") }
	nullPointer := "INSERT INTO tokens (uid,name,created_at,updated_at,spec_lookup,spec_slug,spec_immutable_code,spec_optional_note) VALUES (?,?,?,?,?,?,?,NULL)"
	if _, err := db.ExecContext(t.Context(), nullPointer, "raw-pointer", "raw-pointer", now, now, "lookup-pointer", "raw-pointer", "fixed"); err != nil { t.Fatalf("explicit NULL pointer default failed: %v", err) }
	t.Logf("raw defaults omitted=false/0/fallback scalar_NULL=rejected pointer_NULL=accepted")
}

func TestGeneratedSQLite_CRUD_zero_immutable_unique(t *testing.T) {
	client, db := openGeneratedSQLite(t, filepath.Join(t.TempDir(), "crud.db"))
	t.Cleanup(func() { _ = client.Close() })
	input := acceptanceToken("token-1", "unique-slug")
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if loaded.Spec.Enabled || loaded.Spec.Retries != 0 { t.Fatalf("explicit zero values=%#v", loaded.Spec) }
	input.Spec.Enabled, input.Spec.Retries, input.Spec.ImmutableCode = true, 9, "replacement"
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	input.Spec.Enabled, input.Spec.Retries = false, 0
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	updated, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if updated.Spec.Enabled || updated.Spec.Retries != 0 || updated.Spec.ImmutableCode != "fixed" { t.Fatalf("updated=%#v", updated.Spec) }
	all, err := LoadAllTokens(t.Context())
	if err != nil || len(all) != 1 { t.Fatalf("list=%d err=%v", len(all), err) }
	if count, err := Querytokens(t.Context()).Count(t.Context()); err != nil || count != 1 { t.Fatalf("query count=%d err=%v", count, err) }
	duplicate := acceptanceToken("token-2", "unique-slug")
	if err := SaveToken(t.Context(), duplicate); err == nil { t.Fatal("duplicate unique slug succeeded") }
	if err := DeleteToken(t.Context(), input.Metadata.UID); err != nil { t.Fatal(err) }
	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM tokens").Scan(&count); err != nil { t.Fatal(err) }
	if count != 0 { t.Fatalf("raw rows after delete=%d", count) }
	if err := DeleteToken(t.Context(), "missing"); !errors.Is(err, ErrNotFound) { t.Fatalf("missing delete=%v", err) }
	t.Logf("CRUD zero immutable unique raw_count=%d", count)
}

func TestGeneratedSQLite_dedicated_create_conflict_preserves_ent_cause(t *testing.T) {
	client, _ := openGeneratedSQLite(t, filepath.Join(t.TempDir(), "create-conflict.db"))
	t.Cleanup(func() { _ = client.Close() })
	if err := SaveToken(t.Context(), acceptanceToken("token-1", "duplicate")); err != nil { t.Fatal(err) }

	err := SaveToken(t.Context(), acceptanceToken("token-2", "duplicate"))

	var conflict *StorageConflictError
	if !errors.Is(err, ErrStorageConflict) { t.Fatalf("error=%v, want ErrStorageConflict", err) }
	if !errors.As(err, &conflict) { t.Fatalf("error=%T, want *StorageConflictError", err) }
	if !ent.IsConstraintError(err) { t.Fatalf("error chain lost Ent constraint: %v", err) }
	t.Log("dedicated create conflict chain")
}

func TestGeneratedSQLite_dedicated_update_conflict_preserves_ent_cause(t *testing.T) {
	client, _ := openGeneratedSQLite(t, filepath.Join(t.TempDir(), "update-conflict.db"))
	t.Cleanup(func() { _ = client.Close() })
	first := acceptanceToken("token-1", "first")
	second := acceptanceToken("token-2", "second")
	if err := SaveToken(t.Context(), first); err != nil { t.Fatal(err) }
	if err := SaveToken(t.Context(), second); err != nil { t.Fatal(err) }
	second.Spec.Slug = first.Spec.Slug

	err := SaveToken(t.Context(), second)

	var conflict *StorageConflictError
	if !errors.Is(err, ErrStorageConflict) { t.Fatalf("error=%v, want ErrStorageConflict", err) }
	if !errors.As(err, &conflict) { t.Fatalf("error=%T, want *StorageConflictError", err) }
	if !ent.IsConstraintError(err) { t.Fatalf("error chain lost Ent constraint: %v", err) }
	t.Log("dedicated update conflict chain")
}

func TestGeneratedSQLite_reopen_envelope_and_corrupt_status(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	client, _ := openGeneratedSQLite(t, path)
	want := acceptanceToken("token-reopen", "reopen-slug")
	if err := SaveToken(t.Context(), want); err != nil { t.Fatal(err) }
	if err := client.Close(); err != nil { t.Fatal(err) }
	reopened, reopenedDB := openGeneratedSQLite(t, path)
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := LoadToken(t.Context(), want.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(got, want) { t.Fatalf("reopen got=%#v want=%#v", got, want) }
	if _, err := reopenedDB.ExecContext(t.Context(), "UPDATE tokens SET status=? WHERE uid=?", []byte(` + "`{\"state\":\"corrupt\"}`" + `), want.Metadata.UID); err != nil { t.Fatal(err) }
	corrupt, err := LoadToken(t.Context(), want.Metadata.UID)
	if err == nil || corrupt != nil { t.Fatalf("corrupt status resource=%#v err=%v", corrupt, err) }
	var count int
	if err := reopenedDB.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM tokens").Scan(&count); err != nil { t.Fatal(err) }
	t.Logf("reopen envelope corrupt status raw_count=%d", count)
}
`
