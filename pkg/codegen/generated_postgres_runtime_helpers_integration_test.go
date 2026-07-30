// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

//go:build integration

package codegen

const generatedPostgresRuntimeHelpers = `package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	dialectsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	v1 "example.com/generated-annotation-acceptance/apis/acceptance.example.io/v1"
	"example.com/generated-annotation-acceptance/internal/storage/ent"
	enttoken "example.com/generated-annotation-acceptance/internal/storage/ent/token"
	"github.com/openchami/fabrica/pkg/fabrica"
)

type postgresHarness struct {
	client *ent.Client
	db *sql.DB
	control *sql.DB
	dsn string
	schema string
}

func newPostgresHarness(t *testing.T) *postgresHarness {
	t.Helper()
	baseDSN := os.Getenv("FABRICA_TEST_POSTGRES_DSN")
	if baseDSN == "" { t.Fatal("FABRICA_TEST_POSTGRES_DSN is required") }
	control, err := sql.Open("postgres", baseDSN)
	if err != nil { t.Fatal(err) }
	if err := control.PingContext(t.Context()); err != nil { t.Fatal(err) }
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil { t.Fatal(err) }
	schema := "fabrica_generated_" + hex.EncodeToString(suffix[:])
	if _, err := control.ExecContext(t.Context(), "CREATE SCHEMA "+schema+" AUTHORIZATION CURRENT_USER"); err != nil { t.Fatal(err) }
	u, err := url.Parse(baseDSN)
	if err != nil { t.Fatal(err) }
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	h := &postgresHarness{control: control, dsn: u.String(), schema: schema}
	h.open(t, true)
	t.Cleanup(func() {
		if h.client != nil {
			if err := h.client.Close(); err != nil { t.Errorf("close Ent client: %v", err) }
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := control.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil { t.Errorf("drop schema %s: %v", schema, err) }
		if err := control.Close(); err != nil { t.Errorf("close control DB: %v", err) }
	})
	return h
}

func (h *postgresHarness) open(t *testing.T, migrate bool) {
	t.Helper()
	db, err := sql.Open("postgres", h.dsn)
	if err != nil { t.Fatal(err) }
	if err := db.PingContext(t.Context()); err != nil { t.Fatal(err) }
	client := ent.NewClient(ent.Driver(dialectsql.OpenDB(dialect.Postgres, db)))
	if migrate {
		if err := client.Schema.Create(t.Context()); err != nil { t.Fatal(err) }
	}
	h.client, h.db = client, db
	SetEntClient(client)
}

func (h *postgresHarness) reopen(t *testing.T) {
	t.Helper()
	if err := h.client.Close(); err != nil { t.Fatal(err) }
	h.client, h.db = nil, nil
	h.open(t, false)
}

func postgresToken(uid, slug string) *v1.Token {
	now := time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC)
	note := "optional"
	return &v1.Token{
		APIVersion: "acceptance.example.io/v1", Kind: "Token",
		Metadata: fabrica.Metadata{Name: uid, UID: uid, Namespace: "tenant-a", ResourceVersion: "7", CreatedAt: now, UpdatedAt: now, Labels: map[string]string{"suite": "postgres"}, Annotations: map[string]string{"owner": "qa"}},
		Spec: v1.TokenSpec{Lookup: "lookup-"+uid, Slug: slug, ImmutableCode: "fixed", OptionalNote: &note, Tags: []string{"one", "two"}, Bucket: "bucket-"+uid, Password: "secret-"+uid},
		Status: v1.TokenStatus{State: "ready"},
	}
}

func TestGeneratedPostgresRuntime_role_catalog(t *testing.T) {
	h := newPostgresHarness(t)
	var role string
	var superuser bool
	if err := h.db.QueryRowContext(t.Context(), "SELECT current_user, rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&role, &superuser); err != nil { t.Fatal(err) }
	if superuser { t.Fatalf("runtime role %q is superuser", role) }
	t.Logf("role catalog role=%s rolsuper=%t schema=%s", role, superuser, h.schema)
}

`
