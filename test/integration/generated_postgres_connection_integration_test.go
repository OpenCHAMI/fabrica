// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

const postgresTestDSNEnv = "FABRICA_TEST_POSTGRES_DSN"

func TestGeneratedPostgres_connection_uses_restricted_role(t *testing.T) {
	dsn := requiredPostgresDSN(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL test DSN: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close PostgreSQL baseline connection: %v", err)
		}
	})
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("connect to PostgreSQL test service: %v", err)
	}

	var role string
	var superuser bool
	if err := db.QueryRowContext(ctx, `
		SELECT current_user, rolsuper
		FROM pg_catalog.pg_roles
		WHERE rolname = current_user
	`).Scan(&role, &superuser); err != nil {
		t.Fatalf("inspect PostgreSQL application role: %v", err)
	}
	if superuser {
		t.Fatalf("PostgreSQL integration DSN uses superuser role %q", role)
	}

	schema := uniquePostgresSchema(t)
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("restricted role %q cannot create isolated schema: %v", role, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := db.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
			t.Errorf("drop baseline PostgreSQL schema %s: %v", schema, err)
		}
	})
	t.Logf("restricted PostgreSQL role=%s rolsuper=%t schema=%s", role, superuser, schema)
}

func requiredPostgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(postgresTestDSNEnv)
	if dsn == "" {
		t.Fatalf("%s is required when integration tests are selected", postgresTestDSNEnv)
	}
	return dsn
}

func uniquePostgresSchema(t *testing.T) string {
	t.Helper()
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatalf("generate PostgreSQL schema suffix: %v", err)
	}
	return "fabrica_test_" + hex.EncodeToString(random[:])
}
