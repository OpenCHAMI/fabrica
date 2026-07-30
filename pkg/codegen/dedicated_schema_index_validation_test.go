// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
)

func TestGeneratedDedicatedIndex_sqlite_rejects_postgresql_method_before_rendering(t *testing.T) {
	// Given
	gen, root := newDedicatedIndexGenerator(t, "sqlite", postgresqlDedicatedIndexSource)

	// When
	err := gen.PrepareResourceAnnotations()

	// Then
	if !errors.Is(err, annotations.ErrUnsupportedCapability) {
		t.Fatalf("SQLite PostgreSQL-index error = %v, want ErrUnsupportedCapability", err)
	}
	var capabilityErr *annotations.CapabilityError
	if !errors.As(err, &capabilityErr) || capabilityErr.Capability != annotations.CapabilityIndex {
		t.Fatalf("SQLite PostgreSQL-index error = %#v, want index CapabilityError", capabilityErr)
	}
	if capabilityErr.FieldName != "Tags" || !strings.Contains(capabilityErr.Message, "SQLite") {
		t.Errorf("SQLite capability context = %#v", capabilityErr)
	}
	schemaPath := filepath.Join(root, "internal", "storage", "ent", "schema", "dedicatedindex.go")
	if _, statErr := os.Stat(schemaPath); !os.IsNotExist(statErr) {
		t.Errorf("unsupported SQLite index reached rendering: %v", statErr)
	}
}

func TestGeneratedDedicatedIndex_dialect_change_quarantines_stale_postgresql_schema(t *testing.T) {
	// Given
	gen, root := newDedicatedIndexGenerator(t, "postgres", postgresqlDedicatedIndexSource)
	if err := gen.PrepareResourceAnnotations(); err != nil {
		t.Fatalf("prepare PostgreSQL annotations: %v", err)
	}
	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("generate PostgreSQL schema: %v", err)
	}
	schemaPath := filepath.Join(root, "internal", "storage", "ent", "schema", "dedicatedindex.go")
	before, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read PostgreSQL schema: %v", err)
	}
	gen.DBDriver = "sqlite"

	// When
	err = gen.GenerateEntSchemas()

	// Then
	if !errors.Is(err, annotations.ErrUnsupportedCapability) {
		t.Fatalf("dialect change error = %v, want ErrUnsupportedCapability", err)
	}
	if _, statErr := os.Stat(schemaPath); !os.IsNotExist(statErr) {
		t.Fatalf("stale PostgreSQL schema remains active: %v", statErr)
	}
	quarantineRoot := findTransactionArtifactFixture(t, filepath.Dir(schemaPath), transactionRoleQuarantine)
	quarantinePath := filepath.Join(quarantineRoot, "dedicatedindex.go")
	after, readErr := os.ReadFile(quarantinePath)
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("quarantined PostgreSQL schema differs: %v", readErr)
	}
}

func TestDedicatedSchemaIndex_rejects_malformed_resolved_dialect(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "dedicated_index_types.go")
	writeDedicatedIndexFixtureFile(t, path, postgresqlDedicatedIndexSource)
	resolved, err := annotations.ResolveStorageIntent(path, "DedicatedIndex", annotations.DialectPostgreSQL)
	if err != nil {
		t.Fatalf("resolve valid PostgreSQL state: %v", err)
	}
	resolved.Dialect = annotations.DialectSQLite

	// When
	_, err = buildDedicatedSchemaData(resolved, 2026)

	// Then
	if err == nil || !strings.Contains(err.Error(), "field dialect does not match") {
		t.Fatalf("malformed resolved dialect error = %v", err)
	}
}
