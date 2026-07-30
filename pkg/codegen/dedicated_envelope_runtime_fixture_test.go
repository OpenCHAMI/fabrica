// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

const dedicatedEnvelopeRuntimeTest = `package storage

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"example.com/generated-annotation-acceptance/internal/storage/ent/token"
	"github.com/openchami/fabrica/pkg/fabrica"
)

func openDedicatedEnvelopeDB(t *testing.T, path string) (*ent.Client, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?_fk=1")
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	return client, db
}

func saveDedicatedToken(t *testing.T, client *ent.Client, input *v1.Token) {
	t.Helper()
	create, err := ToEntToken(t.Context(), client, input)
	if err != nil { t.Fatal(err) }
	if _, err := create.Save(t.Context()); err != nil { t.Fatal(err) }
}

func TestDedicatedEnvelope_standard_field_create_read_update_mapping(t *testing.T) {
	// Given
	client, _ := openDedicatedEnvelopeDB(t, filepath.Join(t.TempDir(), "standard.db"))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	input := &v1.Token{
		APIVersion: "v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: "standard", UID: "tok-standard"},
		Spec: v1.TokenSpec{DisplayName: "created", Metadata: "ordinary"},
	}
	saveDedicatedToken(t, client, input)
	entity, err := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }

	// When
	created, err := FromEntToken(t.Context(), entity)
	if err != nil { t.Fatal(err) }
	input.Spec.DisplayName = "updated"
	update := entity.Update()
	if err := UpdateTokenFromResource(t.Context(), update, input); err != nil { t.Fatal(err) }
	updatedEntity, err := update.Save(t.Context())
	if err != nil { t.Fatal(err) }
	updated, err := FromEntToken(t.Context(), updatedEntity)
	if err != nil { t.Fatal(err) }

	// Then
	if created.Spec.DisplayName != "created" || created.Spec.Metadata != "ordinary" {
		t.Fatalf("create/read mapping = %#v", created.Spec)
	}
	if updated.Spec.DisplayName != "updated" || updated.Spec.Metadata != "ordinary" {
		t.Fatalf("update mapping = %#v", updated.Spec)
	}
	t.Log("standard field mapping")
}

func TestDedicatedEnvelope_reload_round_trip(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "envelope.db")
	client, _ := openDedicatedEnvelopeDB(t, dbPath)
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	createdAt := time.Date(2026, 7, 1, 2, 3, 4, 0, time.UTC)
	updatedAt := createdAt.Add(5 * time.Minute)
	want := &v1.Token{
		APIVersion: "acceptance.example.io/v1",
		Kind: "Token",
		Metadata: fabrica.Metadata{
			Name: "namespaced-token", UID: "tok-namespaced", Namespace: "tenant-a",
			ResourceVersion: "17", CreatedAt: createdAt, UpdatedAt: updatedAt,
			Labels: map[string]string{"environment": "test"},
			Annotations: map[string]string{"owner": "qa"},
		},
		Spec: v1.TokenSpec{DisplayName: "display", Metadata: "spec metadata"},
		Status: v1.TokenStatus{State: "ready"},
	}
	saveDedicatedToken(t, client, want)
	saved, err := client.Token.Query().Where(token.UIDEQ(want.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }
	want.Metadata.Name = "renamed-token"
	want.Metadata.ResourceVersion = "18"
	want.Metadata.UpdatedAt = updatedAt.Add(time.Minute)
	want.Metadata.Labels = map[string]string{"environment": "updated"}
	want.Metadata.Annotations = map[string]string{"owner": "runtime"}
	want.Spec = v1.TokenSpec{DisplayName: "updated display", Metadata: "updated metadata"}
	want.Status = v1.TokenStatus{State: "updated"}
	update := saved.Update()
	if err := UpdateTokenFromResource(t.Context(), update, want); err != nil { t.Fatal(err) }
	if _, err := update.Save(t.Context()); err != nil { t.Fatal(err) }
	if err := client.Close(); err != nil { t.Fatal(err) }

	// When
	reopened, _ := openDedicatedEnvelopeDB(t, dbPath)
	t.Cleanup(func() { _ = reopened.Close() })
	entity, err := reopened.Token.Query().Where(token.UIDEQ(want.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }
	got, err := FromEntToken(t.Context(), entity)
	if err != nil { t.Fatal(err) }

	// Then
	if !reflect.DeepEqual(got, want) { t.Fatalf("reload round-trip mismatch\ngot: %#v\nwant: %#v", got, want) }
	t.Log("reload round-trip")
}

func TestDedicatedEnvelope_preserves_nil_and_empty_maps(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "maps.db")
	client, _ := openDedicatedEnvelopeDB(t, dbPath)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	inputs := []*v1.Token{
		{APIVersion: "v1", Kind: "Token", Metadata: fabrica.Metadata{Name: "nil", UID: "tok-nil"}, Spec: v1.TokenSpec{DisplayName: "nil", Metadata: "nil"}},
		{APIVersion: "v1", Kind: "Token", Metadata: fabrica.Metadata{Name: "empty", UID: "tok-empty", Labels: map[string]string{}, Annotations: map[string]string{}}, Spec: v1.TokenSpec{DisplayName: "empty", Metadata: "empty"}},
	}
	for _, input := range inputs { saveDedicatedToken(t, client, input) }

	// When
	nilEntity, err := client.Token.Query().Where(token.UIDEQ("tok-nil")).Only(t.Context())
	if err != nil { t.Fatal(err) }
	emptyEntity, err := client.Token.Query().Where(token.UIDEQ("tok-empty")).Only(t.Context())
	if err != nil { t.Fatal(err) }
	nilResource, err := FromEntToken(t.Context(), nilEntity)
	if err != nil { t.Fatal(err) }
	emptyResource, err := FromEntToken(t.Context(), emptyEntity)
	if err != nil { t.Fatal(err) }

	// Then
	if nilResource.Metadata.Labels != nil || nilResource.Metadata.Annotations != nil { t.Fatalf("nil maps normalized unexpectedly: %#v", nilResource.Metadata) }
	if emptyResource.Metadata.Labels == nil || emptyResource.Metadata.Annotations == nil { t.Fatalf("empty maps became nil: %#v", emptyResource.Metadata) }
	if nilResource.Metadata.Namespace != "" || emptyResource.Metadata.Namespace != "" { t.Fatalf("cluster-scoped namespace changed: %#v %#v", nilResource.Metadata, emptyResource.Metadata) }
	t.Log("nil and empty maps")
}

func TestDedicatedEnvelope_corrupt_status_returns_contextual_error(t *testing.T) {
	// Given
	dbPath := filepath.Join(t.TempDir(), "corrupt.db")
	client, db := openDedicatedEnvelopeDB(t, dbPath)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	input := &v1.Token{APIVersion: "v1", Kind: "Token", Metadata: fabrica.Metadata{Name: "corrupt", UID: "tok-corrupt"}, Spec: v1.TokenSpec{DisplayName: "corrupt", Metadata: "corrupt"}}
	saveDedicatedToken(t, client, input)
	if _, err := db.ExecContext(t.Context(), "UPDATE tokens SET status = ? WHERE uid = ?", []byte(` + "`{`" + `), input.Metadata.UID); err != nil { t.Fatal(err) }
	entity, queryErr := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if queryErr != nil {
		if !strings.Contains(strings.ToLower(queryErr.Error()), "status") { t.Fatalf("query corrupt status error lacks context: %v", queryErr) }
		t.Log("corrupt status")
		return
	}

	// When
	got, err := FromEntToken(t.Context(), entity)

	// Then
	if err == nil || !strings.Contains(err.Error(), "unmarshal Token status") { t.Fatalf("error = %v, want contextual corrupt status error", err) }
	if got != nil { t.Fatalf("corrupt status returned partial resource: %#v", got) }
	t.Log("corrupt status")
}

func TestDedicatedEnvelope_status_marshal_error_is_checked(t *testing.T) {
	// Given
	client, _ := openDedicatedEnvelopeDB(t, filepath.Join(t.TempDir(), "marshal.db"))
	t.Cleanup(func() { _ = client.Close() })
	input := &v1.Token{Status: v1.TokenStatus{FailMarshal: true}}

	// When
	create, err := ToEntToken(t.Context(), client, input)

	// Then
	if err == nil || !strings.Contains(err.Error(), "marshal Token status") { t.Fatalf("error = %v, want contextual marshal failure", err) }
	if create != nil { t.Fatalf("marshal failure returned partial create builder: %#v", create) }
}
`
