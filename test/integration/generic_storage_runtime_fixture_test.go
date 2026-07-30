// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const genericStorageRuntimeTest = `package storage

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"github.com/openchami/fabrica/pkg/fabrica"
)

func openGenericStorage(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	SetEntClient(client)
	return client, db
}

func genericToken() *v1.Token {
	createdAt := time.Date(2026, 7, 1, 2, 3, 4, 0, time.UTC)
	return &v1.Token{
		APIVersion: "acceptance.example.io/v1",
		Kind: "Token",
		Metadata: fabrica.Metadata{
			Name: "generic", UID: "generic-1", Namespace: "tenant-a",
			ResourceVersion: "7",
			CreatedAt: createdAt, UpdatedAt: createdAt,
			Labels: map[string]string{"environment": "test"},
			Annotations: map[string]string{"owner": "qa"},
		},
		Spec: v1.TokenSpec{Value: "created"},
		Status: v1.TokenStatus{State: "ready"},
	}
}

func TestGenericStorage_CRUD_uses_resource_table(t *testing.T) {
	// Given
	client, _ := openGenericStorage(t)
	input := genericToken()
	input.Metadata.Labels = nil
	input.Metadata.Annotations = nil

	// When
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	input.Spec.Value = "updated"
	input.Metadata.Name = "renamed"
	input.Metadata.UpdatedAt = input.Metadata.UpdatedAt.Add(time.Minute)
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	all, err := LoadAllTokens(t.Context())
	if err != nil { t.Fatal(err) }
	count, err := client.Resource.Query().Count(t.Context())
	if err != nil { t.Fatal(err) }

	// Then
	if count != 1 || len(all) != 1 { t.Fatalf("generic rows=%d all=%d, want 1", count, len(all)) }
	if loaded.Spec.Value != "updated" || loaded.Metadata.Name != "renamed" { t.Fatalf("loaded = %#v", loaded) }
	if !loaded.Metadata.CreatedAt.Equal(genericToken().Metadata.CreatedAt) { t.Fatalf("createdAt changed: %s", loaded.Metadata.CreatedAt) }
	if err := DeleteToken(t.Context(), input.Metadata.UID); err != nil { t.Fatal(err) }
	t.Log("generic CRUD")
}

func TestGenericStorage_query_helpers_keep_public_contract(t *testing.T) {
	// Given
	client, _ := openGenericStorage(t)
	input := genericToken()
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }

	// When
	byUID, err := GetTokenByUID(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	byLabels, err := ListtokensByLabels(t.Context(), map[string]string{"environment": "test"})
	if err != nil { t.Fatal(err) }
	count, err := Querytokens(t.Context()).Count(t.Context())
	if err != nil { t.Fatal(err) }
	genericCount, err := client.Resource.Query().Count(t.Context())
	if err != nil { t.Fatal(err) }

	// Then
	if byUID.Metadata.UID != input.Metadata.UID || len(byLabels) != 1 || count != 1 || genericCount != 1 {
		t.Fatalf("uid=%q labels=%d query=%d generic=%d", byUID.Metadata.UID, len(byLabels), count, genericCount)
	}
	t.Log("generic query helpers")
}

func TestGenericStorage_delete_preserves_not_found_parity(t *testing.T) {
	// Given
	openGenericStorage(t)

	// When
	err := DeleteToken(t.Context(), "missing")

	// Then
	if !errors.Is(err, ErrNotFound) { t.Fatalf("error = %v, want ErrNotFound", err) }
	t.Log("generic not found")
}

func TestGenericStorage_update_persists_zero_and_changed_metadata(t *testing.T) {
	// Given
	_, db := openGenericStorage(t)
	input := genericToken()
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }

	// When
	input.Metadata.Namespace = ""
	input.Metadata.ResourceVersion = ""
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }

	// Then
	assertGenericMetadata := func(wantNamespace, wantResourceVersion string) {
		t.Helper()
		var rawNamespace, rawResourceVersion string
		if err := db.QueryRowContext(t.Context(), "SELECT namespace, resource_version FROM resources WHERE uid = ?", input.Metadata.UID).Scan(&rawNamespace, &rawResourceVersion); err != nil { t.Fatal(err) }
		loaded, err := LoadToken(t.Context(), input.Metadata.UID)
		if err != nil { t.Fatal(err) }
		if rawNamespace != wantNamespace || rawResourceVersion != wantResourceVersion { t.Fatalf("raw metadata=(%q,%q), want (%q,%q)", rawNamespace, rawResourceVersion, wantNamespace, wantResourceVersion) }
		if loaded.Metadata.Namespace != wantNamespace || loaded.Metadata.ResourceVersion != wantResourceVersion { t.Fatalf("loaded metadata=(%q,%q), want (%q,%q)", loaded.Metadata.Namespace, loaded.Metadata.ResourceVersion, wantNamespace, wantResourceVersion) }
	}
	assertGenericMetadata("", "")

	input.Metadata.Namespace = "tenant-b"
	input.Metadata.ResourceVersion = "8"
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	assertGenericMetadata("tenant-b", "8")
	t.Log("generic metadata zero and changed")
}

func TestGenericStorage_create_conflict_preserves_ent_cause_when_unique_index_applies(t *testing.T) {
	_, db := openGenericStorage(t)
	if _, err := db.ExecContext(t.Context(), "CREATE UNIQUE INDEX resources_spec_value_unique ON resources(json_extract(spec, '$.value'))"); err != nil { t.Fatal(err) }
	first := genericToken()
	if err := SaveToken(t.Context(), first); err != nil { t.Fatal(err) }
	second := genericToken()
	second.Metadata.UID = "generic-2"

	err := SaveToken(t.Context(), second)

	var conflict *StorageConflictError
	if !errors.Is(err, ErrStorageConflict) { t.Fatalf("error=%v, want ErrStorageConflict", err) }
	if !errors.As(err, &conflict) { t.Fatalf("error=%T, want *StorageConflictError", err) }
	if !ent.IsConstraintError(err) { t.Fatalf("error chain lost Ent constraint: %v", err) }
	t.Log("generic create conflict chain")
}

func TestGenericStorage_update_conflict_preserves_ent_cause_when_unique_index_applies(t *testing.T) {
	_, db := openGenericStorage(t)
	if _, err := db.ExecContext(t.Context(), "CREATE UNIQUE INDEX resources_spec_value_unique ON resources(json_extract(spec, '$.value'))"); err != nil { t.Fatal(err) }
	first := genericToken()
	first.Spec.Value = "first"
	second := genericToken()
	second.Metadata.UID = "generic-2"
	second.Spec.Value = "second"
	if err := SaveToken(t.Context(), first); err != nil { t.Fatal(err) }
	if err := SaveToken(t.Context(), second); err != nil { t.Fatal(err) }
	second.Spec.Value = first.Spec.Value

	err := SaveToken(t.Context(), second)

	var conflict *StorageConflictError
	if !errors.Is(err, ErrStorageConflict) { t.Fatalf("error=%v, want ErrStorageConflict", err) }
	if !errors.As(err, &conflict) { t.Fatalf("error=%T, want *StorageConflictError", err) }
	if !ent.IsConstraintError(err) { t.Fatalf("error chain lost Ent constraint: %v", err) }
	t.Log("generic update conflict chain")
}
`
