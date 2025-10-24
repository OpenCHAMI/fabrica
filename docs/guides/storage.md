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
import "github.com/alexlovelltroy/fabrica/pkg/storage"

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

**Questions?** [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
