// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package integration

const dedicatedMigrationRuntimeHelpers = `package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	"example.com/generated-annotation-acceptance/internal/storage/ent/token"
	"github.com/openchami/fabrica/pkg/fabrica"
)

func openMigrationDB(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "migration.db")) + "?_fk=1"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	SetEntClient(client)
	return client, db
}

type commitFailDriver struct {
	dialect.Driver
	failCommit *bool
}

func (driver commitFailDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := driver.Driver.Tx(ctx)
	if err != nil { return nil, err }
	return commitFailTx{Tx: tx, failCommit: driver.failCommit}, nil
}

type commitFailTx struct {
	dialect.Tx
	failCommit *bool
}

func (tx commitFailTx) Commit() error {
	if !*tx.failCommit { return tx.Tx.Commit() }
	if err := tx.Tx.Rollback(); err != nil { return errors.Join(errors.New("injected commit failure"), err) }
	return errors.New("injected commit failure")
}

func openCommitFailureMigrationDB(t *testing.T) (*ent.Client, *sql.DB) {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "migration.db")) + "?_fk=1"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil { t.Fatal(err) }
	failCommit := false
	driver := commitFailDriver{Driver: dialectsql.OpenDB(dialect.SQLite, db), failCommit: &failCommit}
	client := ent.NewClient(ent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	failCommit = true
	SetEntClient(client)
	return client, db
}

func genericToken(uid, password string) *v1.Token {
	now := time.Date(2026, 7, 13, 1, 2, 3, 0, time.UTC)
	return &v1.Token{
		APIVersion: "acceptance.example.io/v1", Kind: "Token",
		Metadata: fabrica.Metadata{
			Name: uid + "-name", UID: uid, Namespace: "tenant-a",
			CreatedAt: now, UpdatedAt: now, ResourceVersion: "7",
			Labels: map[string]string{"source": "generic"},
			Annotations: map[string]string{"migration": "required"},
		},
		Spec: v1.TokenSpec{DisplayName: uid + "-display", Password: password, SensitiveNote: "private-" + uid},
		Status: v1.TokenStatus{State: "ready"},
	}
}

func seedGenericToken(t *testing.T, client *ent.Client, input *v1.Token) {
	t.Helper()
	spec, err := json.Marshal(input.Spec)
	if err != nil { t.Fatal(err) }
	status, err := json.Marshal(input.Status)
	if err != nil { t.Fatal(err) }
	entity, err := client.Resource.Create().
		SetUID(input.Metadata.UID).SetName(input.Metadata.Name).
		SetAPIVersion(input.APIVersion).SetKind(input.Kind).SetResourceType(input.Kind).
		SetSpec(spec).SetStatus(status).SetCreatedAt(input.Metadata.CreatedAt).SetUpdatedAt(input.Metadata.UpdatedAt).
		SetResourceVersion(input.Metadata.ResourceVersion).SetNamespace(input.Metadata.Namespace).Save(t.Context())
	if err != nil { t.Fatal(err) }
	for key, value := range input.Metadata.Labels {
		if _, err := client.Label.Create().SetKey(key).SetValue(value).SetResourceID(entity.ID).Save(t.Context()); err != nil { t.Fatal(err) }
	}
	for key, value := range input.Metadata.Annotations {
		if _, err := client.Annotation.Create().SetKey(key).SetValue(value).SetResourceID(entity.ID).Save(t.Context()); err != nil { t.Fatal(err) }
	}
}

func rawMigrationCounts(t *testing.T, db *sql.DB) (int, int) {
	t.Helper()
	var generic int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM resources WHERE kind = 'Token'").Scan(&generic); err != nil { t.Fatal(err) }
	var dedicated int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM tokens").Scan(&dedicated); err != nil { t.Fatal(err) }
	return generic, dedicated
}

func rawDedicatedUIDs(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "SELECT uid FROM tokens ORDER BY uid")
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	uids := make([]string, 0)
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil { t.Fatal(err) }
		uids = append(uids, uid)
	}
	if err := rows.Err(); err != nil { t.Fatal(err) }
	return uids
}

type genericSourceSnapshot struct {
	Resources [][][]byte
	Labels [][][]byte
	Annotations [][][]byte
}

func snapshotGenericSources(t *testing.T, db *sql.DB) genericSourceSnapshot {
	t.Helper()
	return genericSourceSnapshot{
		Resources: snapshotRawRows(t, db, "SELECT id, uid, name, api_version, kind, resource_type, spec, status, created_at, updated_at, resource_version, namespace FROM resources ORDER BY id"),
		Labels: snapshotRawRows(t, db, "SELECT id, key, value, resource_labels FROM labels ORDER BY id"),
		Annotations: snapshotRawRows(t, db, "SELECT id, key, value, resource_annotations FROM annotations ORDER BY id"),
	}
}

func snapshotRawRows(t *testing.T, db *sql.DB, query string) [][][]byte {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query)
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil { t.Fatal(err) }
	result := make([][][]byte, 0)
	for rows.Next() {
		values := make([]sql.RawBytes, len(columns))
		destinations := make([]any, len(columns))
		for index := range values { destinations[index] = &values[index] }
		if err := rows.Scan(destinations...); err != nil { t.Fatal(err) }
		row := make([][]byte, len(values))
		for index, value := range values {
			if value != nil { row[index] = append([]byte(nil), value...) }
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil { t.Fatal(err) }
	return result
}

func requireGenericSourcesUnchanged(t *testing.T, stage string, before, after genericSourceSnapshot) {
	t.Helper()
	beforeDigest := genericSourceDigest(t, before)
	afterDigest := genericSourceDigest(t, after)
	t.Logf("generic source %s rows=%d/%d/%d sha256=%s", stage, len(after.Resources), len(after.Labels), len(after.Annotations), afterDigest)
	if !reflect.DeepEqual(before, after) { t.Fatalf("generic source changed during %s: before_sha256=%s after_sha256=%s", stage, beforeDigest, afterDigest) }
}

func genericSourceDigest(t *testing.T, snapshot genericSourceSnapshot) string {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil { t.Fatal(err) }
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func requireReport(t *testing.T, report DedicatedMigrationReport, scanned, eligible, copied, skipped int) {
	t.Helper()
	if report.Scanned != scanned || report.Eligible != eligible || report.Copied != copied || report.Skipped != skipped {
		t.Fatalf("report=%#v want scanned=%d eligible=%d copied=%d skipped=%d", report, scanned, eligible, copied, skipped)
	}
}

func requireMigratedToken(t *testing.T, client *ent.Client, source *v1.Token) {
	t.Helper()
	entity, err := client.Token.Query().Where(token.UIDEQ(source.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }
	if entity.SpecPassword == source.Spec.Password { t.Fatal("migration persisted plaintext bcrypt input") }
	if err := bcrypt.CompareHashAndPassword([]byte(entity.SpecPassword), []byte(source.Spec.Password)); err != nil { t.Fatal(err) }
	loaded, err := LoadToken(t.Context(), source.Metadata.UID)
	if err != nil { t.Fatal(err) }
	if loaded.Metadata.Name != source.Metadata.Name || loaded.Metadata.Namespace != source.Metadata.Namespace || loaded.Metadata.ResourceVersion != source.Metadata.ResourceVersion { t.Fatalf("metadata=%#v", loaded.Metadata) }
	if loaded.Metadata.Labels["source"] != "generic" || loaded.Metadata.Annotations["migration"] != "required" { t.Fatalf("metadata maps=%#v", loaded.Metadata) }
	if loaded.Status.State != "ready" || loaded.Spec.DisplayName != source.Spec.DisplayName { t.Fatalf("loaded=%#v", loaded) }
	if loaded.Spec.Password != "" || loaded.Spec.SensitiveNote != "" { t.Fatalf("sensitive fields leaked: %#v", loaded.Spec) }
}

`
