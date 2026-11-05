<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Storage System Guide

> Pluggable storage backends for flexible resource persistence.

## Table of Contents

- [Overview](#overview)
- [Storage Interface](#storage-interface)
- [File Backend](#file-backend)
- [Custom Backends](#custom-backends)
- [Best Practices](#best-practices)

## Overview

Fabrica provides a pluggable storage system that allows you to persist resources in different backends without changing your application code.

**Built-in:**
- 📁 File-based storage (JSON files, great for development)
- �️ Ent backend (SQLite, PostgreSQL, MySQL for production)

**Planned:**
- ☁️ Cloud storage backends (S3, GCS)

## Storage Interface

All storage backends implement the `StorageBackend` interface:

```go
type StorageBackend interface {
    LoadAll(ctx context.Context, resourceType string) ([]json.RawMessage, error)
    Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error)
    Save(ctx context.Context, resourceType, uid string, data json.RawMessage) error
    Delete(ctx context.Context, resourceType, uid string) error
    Exists(ctx context.Context, resourceType, uid string) (bool, error)
    List(ctx context.Context, resourceType string) ([]string, error)
    Close() error

    // Version support
    LoadWithVersion(ctx context.Context, resourceType, uid, version string) (json.RawMessage, string, error)
    LoadAllWithVersion(ctx context.Context, resourceType, version string) ([]json.RawMessage, error)
    SaveWithVersion(ctx context.Context, resourceType, uid string, data json.RawMessage, version string) error
}
```

## File Backend

The default file-based storage backend stores resources as JSON files.

### Basic Usage

```go
import "github.com/openchami/fabrica/pkg/storage"

// Create backend
backend := storage.NewFileBackend("./data")
defer backend.Close()

// Use with generated storage
storage := NewResourceStorage[*Device](backend, "Device")
```

### Directory Structure

```
data/
├── Device/
│   ├── dev-1a2b3c4d.json
│   ├── dev-2b3c4d5e.json
│   └── dev-3c4d5e6f.json
├── User/
│   ├── usr-a1b2c3d4.json
│   └── usr-b2c3d4e5.json
└── Product/
    └── prd-1234abcd.json
```

### File Format

Each resource is stored as a JSON file:

```json
{
  "apiVersion": "v1",
  "kind": "Device",
  "metadata": {
    "uid": "dev-1a2b3c4d",
    "name": "sensor-01",
    "labels": {
      "location": "warehouse-a"
    },
    "createdAt": "2024-10-03T10:00:00Z",
    "updatedAt": "2024-10-03T10:00:00Z"
  },
  "spec": {
    "name": "Temperature Sensor",
    "type": "sensor"
  },
  "status": {
    "online": true
  }
}
```

### Configuration

```go
// Custom data directory
backend := storage.NewFileBackend("/var/lib/myapp/data")

// Multiple backends for different resource types
deviceBackend := storage.NewFileBackend("./devices")
userBackend := storage.NewFileBackend("./users")
```

### Operations

**Create/Update:**
```go
ctx := context.Background()

device := &Device{
    // ... populate fields
}

// Save (creates or updates)
err := backend.Save(ctx, "Device", device.GetUID(), deviceJSON)
```

**Read:**
```go
// Load single resource
data, err := backend.Load(ctx, "Device", "dev-1a2b3c4d")

// Load all resources
allData, err := backend.LoadAll(ctx, "Device")

// Check existence
exists, err := backend.Exists(ctx, "Device", "dev-1a2b3c4d")

// List UIDs
uids, err := backend.List(ctx, "Device")
```

**Delete:**
```go
err := backend.Delete(ctx, "Device", "dev-1a2b3c4d")
```

### Thread Safety

File backend is thread-safe and can be used concurrently:

```go
// Safe to use from multiple goroutines
go func() {
    backend.Save(ctx, "Device", "dev-1", data1)
}()

go func() {
    backend.Save(ctx, "Device", "dev-2", data2)
}()
```

## Custom Backends

Implement the `StorageBackend` interface for custom storage.

Backends plugged into generated adapters with `storage.Init(backend)` can also
implement the optional `AtomicMutationBackend` interface for conditional writes.
The backend must coordinate mutations with ordinary saves and deletes, and must
commit the precondition check and resource change atomically. Without that
capability, conditional mutation requests return `501 Not Implemented`;
unconditional requests retain their existing behavior. See
[atomic generated mutations](conditional-and-patch.md#atomic-generated-mutations)
for callback requirements, handwritten storage mode, and regeneration steps.

### PostgreSQL Example

```go
package storage

import (
    "context"
    "database/sql"
    "encoding/json"

    _ "github.com/lib/pq"
)

type PostgresBackend struct {
    db *sql.DB
}

func NewPostgresBackend(connectionString string) (*PostgresBackend, error) {
    db, err := sql.Open("postgres", connectionString)
    if err != nil {
        return nil, err
    }

    // Create schema
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS resources (
            resource_type VARCHAR(255) NOT NULL,
            uid VARCHAR(255) NOT NULL,
            data JSONB NOT NULL,
            created_at TIMESTAMP NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
            PRIMARY KEY (resource_type, uid)
        )
    `)
    if err != nil {
        return nil, err
    }

    return &PostgresBackend{db: db}, nil
}

func (b *PostgresBackend) Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error) {
    var data json.RawMessage

    err := b.db.QueryRowContext(ctx,
        "SELECT data FROM resources WHERE resource_type = $1 AND uid = $2",
        resourceType, uid,
    ).Scan(&data)

    if err == sql.ErrNoRows {
        return nil, storage.ErrNotFound
    }

    return data, err
}

func (b *PostgresBackend) Save(ctx context.Context, resourceType, uid string, data json.RawMessage) error {
    _, err := b.db.ExecContext(ctx, `
        INSERT INTO resources (resource_type, uid, data, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        ON CONFLICT (resource_type, uid)
        DO UPDATE SET data = $3, updated_at = NOW()
    `, resourceType, uid, data)

    return err
}

func (b *PostgresBackend) Delete(ctx context.Context, resourceType, uid string) error {
    result, err := b.db.ExecContext(ctx,
        "DELETE FROM resources WHERE resource_type = $1 AND uid = $2",
        resourceType, uid,
    )
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rows == 0 {
        return storage.ErrNotFound
    }

    return nil
}

func (b *PostgresBackend) LoadAll(ctx context.Context, resourceType string) ([]json.RawMessage, error) {
    rows, err := b.db.QueryContext(ctx,
        "SELECT data FROM resources WHERE resource_type = $1 ORDER BY created_at",
        resourceType,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var results []json.RawMessage
    for rows.Next() {
        var data json.RawMessage
        if err := rows.Scan(&data); err != nil {
            continue // Skip corrupted data
        }
        results = append(results, data)
    }

    return results, nil
}

func (b *PostgresBackend) Exists(ctx context.Context, resourceType, uid string) (bool, error) {
    var exists bool
    err := b.db.QueryRowContext(ctx,
        "SELECT EXISTS(SELECT 1 FROM resources WHERE resource_type = $1 AND uid = $2)",
        resourceType, uid,
    ).Scan(&exists)

    return exists, err
}

func (b *PostgresBackend) List(ctx context.Context, resourceType string) ([]string, error) {
    rows, err := b.db.QueryContext(ctx,
        "SELECT uid FROM resources WHERE resource_type = $1 ORDER BY created_at",
        resourceType,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var uids []string
    for rows.Next() {
        var uid string
        if err := rows.Scan(&uid); err != nil {
            continue
        }
        uids = append(uids, uid)
    }

    return uids, nil
}

func (b *PostgresBackend) Close() error {
    return b.db.Close()
}

// Implement versioning methods...
func (b *PostgresBackend) LoadWithVersion(ctx context.Context, resourceType, uid, version string) (json.RawMessage, string, error) {
    // Implementation
    return nil, "", nil
}

func (b *PostgresBackend) LoadAllWithVersion(ctx context.Context, resourceType, version string) ([]json.RawMessage, error) {
    // Implementation
    return nil, nil
}

func (b *PostgresBackend) SaveWithVersion(ctx context.Context, resourceType, uid string, data json.RawMessage, version string) error {
    // Implementation
    return nil
}
```

### Usage

```go
// Use PostgreSQL backend
backend, err := storage.NewPostgresBackend("postgres://user:pass@localhost/mydb")
if err != nil {
    log.Fatal(err)
}
defer backend.Close()

// Use with generated storage
deviceStorage := NewResourceStorage[*Device](backend, "Device")
```

## Best Practices

### Error Handling

```go
✅ Check for ErrNotFound specifically
✅ Use context for timeouts
✅ Log storage errors
✅ Handle corrupted data gracefully

// Good
device, err := storage.Load(ctx, uid)
if errors.Is(err, storage.ErrNotFound) {
    return http.StatusNotFound
}
if err != nil {
    log.Error("storage error", "error", err)
    return http.StatusInternalServerError
}
```

### Context Usage

```go
✅ Always use context
✅ Set reasonable timeouts
✅ Handle cancellation

// Good
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

devices, err := storage.LoadAll(ctx)
```

### Performance

```go
✅ Use List() instead of LoadAll() when you only need UIDs
✅ Implement caching for read-heavy workloads
✅ Batch operations when possible
✅ Add indexes for queries

// Good - Only need UIDs
uids, err := storage.List(ctx)

// Bad - Loading full resources
devices, err := storage.LoadAll(ctx)
for _, d := range devices {
    uids = append(uids, d.GetUID())
}
```

### File Backend Specific

```go
✅ Use absolute paths for data directory
✅ Ensure write permissions
✅ Regular backups
✅ Monitor disk space

// Good
absPath, _ := filepath.Abs("./data")
backend := storage.NewFileBackend(absPath)

// Check permissions
if err := os.MkdirAll(absPath, 0755); err != nil {
    log.Fatal("Cannot create data directory:", err)
}
```

## Summary

Fabrica storage provides:

- 🔌 **Pluggable** - Swap backends without code changes
- 📁 **File backend** - Production-ready default
- 🔒 **Thread-safe** - Concurrent access supported
- ⚡ **Efficient** - Optimized operations
- 🎯 **Type-safe** - Compile-time checking

**Next Steps:**
- Use file backend for development
- Implement PostgreSQL for production
- Add caching layer
- Monitor storage metrics

---

**Questions?** [GitHub Issues](https://github.com/openchami/fabrica/issues)

## Regenerate-safe feature startup

New services use a generated feature loader so feature selections can change
without rewriting user-owned `cmd/server/main.go`. See the
[feature loader guide](../guides/feature-loader.md) for regeneration commands,
selective-generation rules, customization boundaries, and migration of existing
services.

### Source annotations and dedicated Ent tables

`fabrica generate` reads annotations from registered resource types and their
`<Name>Spec` companions, including declarations split across Go files. Put
`+fabrica:resource` and `+fabrica:storage=dedicated` on the resource declaration;
put field controls on Spec fields. Dedicated tables require `--storage-type ent`.
Source packages must be available inside the project module when generating.
Invalid controls and unimplemented encryption/Argon2 emission fail generation
with the resource name.
Unannotated resources continue to use shared generic storage.

Parsing and schema emission alone do not establish runtime support. When
upgrading an existing service, review regenerated storage and plan its data
migration before enabling dedicated tables; existing generic rows are not
moved automatically.

Dedicated Ent columns use the JSON tag as their schema name; the adapter maps
it back to the Go field name. JSON names must form distinct Ent identifiers
and must not collide with metadata or Ent's built-in ID. Non-ASCII names are
rejected because Ent v0.14.6's identifier capitalization cannot handle them
reliably. Snake case and camel case names are supported.

Dedicated tables keep Spec values in their flattened columns and store the
remaining API envelope in an optional JSON `envelope` column. This preserves
status, labels, annotations and legacy envelope fields across round trips.
Spec is excluded from that JSON column so hashed credentials are not also
persisted as plaintext. Identity columns remain authoritative on reads.
Existing dedicated rows without an envelope retain their column values;
previously discarded metadata/status cannot be reconstructed automatically.

### Dedicated storage operations and upgrades

Typed CRUD, `StorageBackend`, `AtomicMutationBackend` and the reconciliation
client use the resource's selected table. Saves, conditional updates and deletes
share transactional compare-and-swap checks. Mutations return the stored
representation after defaults, hashing and immutable-field handling. An unchanged
stored hash is not hashed again during an unrelated update.

Per-resource query builders return that resource's Ent query type. For example,
`Querytokens` returns `*ent.TokenQuery` for a dedicated Token. Use
`ListtokensByLabels` or `QuerytokensByLabels` for its labels. The generic
`QueryResources` builder is only for generic kinds and panics with guidance if
asked for a dedicated kind; its error-returning label helper reports the same
restriction. Existing generic resource builders retain their types.

Changing an existing kind from generic to dedicated storage requires a planned
migration. Reads and writes return `storage.ErrDedicatedMigrationRequired` if
rows of that kind still exist in the generic table. Query builders that cannot
return an error panic with this diagnostic. The guard adds a generic-table
existence query to dedicated operations. It never moves or deletes legacy rows.

Before upgrading, stop writers and back up the database. For ordinary resources,
first regenerate the existing generic service with this generator so its export
command emits API envelopes rather than raw Ent rows. Export those envelopes,
then import into a separate empty database with the dedicated schemas. Verify UIDs, counts,
Spec, metadata and status before switching the service to that database. Keep the
old database and service version for rollback. Test this process on a copy first;
changing the annotation on the live database is not a migration procedure.

JSON/YAML export and import preserve the envelope for dedicated resources without
hashed fields. Credential resources with hashed fields are rejected during
import/export preflight: passing an exported hash through ordinary create hooks
would hash it again. Use a full database backup/restore to preserve those hashes;
a portable trusted credential migration format is not implemented. Revision
resources retain their existing database-backup requirement.
