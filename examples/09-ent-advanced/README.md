<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 09: Advanced Ent Storage Features

**What you'll build:** A system management API with advanced query patterns, atomic transactions, and data export/import capabilities using Ent storage.

**What you'll learn:**
- Query builders for efficient database queries with label filtering
- Atomic transactions for multi-resource operations
- Export/import workflows for backups and data migration
- Building complex queries with Ent's fluent API
- Practical patterns for microservice storage layers

**Difficulty:** ⭐⭐⭐ Intermediate
**Time:** 30-45 minutes
**Database:** SQLite (dev) / PostgreSQL (prod)

## Overview

This example demonstrates real-world patterns for managing resources in a production system:

```
┌─────────────────────────────┐
│   HTTP Handlers             │
│  - Query by labels          │
│  - Atomic creates           │
│  - Bulk operations          │
└──────────┬──────────────────┘
           │
           ↓
┌─────────────────────────────┐
│   Query Builders            │
│  - QueryServers()           │
│  - ListServersByLabels()    │
│  - GetServerByUID()         │
└──────────┬──────────────────┘
           │
           ↓
┌─────────────────────────────┐
│   Transactions              │
│  - Atomic multi-resource    │
│  - Rollback on error        │
│  - Consistency guarantees   │
└──────────┬──────────────────┘
           │
           ↓
┌─────────────────────────────┐
│   Ent Storage Backend       │
│  - PostgreSQL/SQLite        │
│  - Automatic migrations     │
│  - Type-safe queries        │
└─────────────────────────────┘
```

## Quick Start

### Option A: Automated Demo (recommended)

```bash
cd examples/09-ent-advanced
bash quick-start.sh
```

This script:
1. Creates a demo project
2. Adds resources (Server, ServerConfig)
3. Generates all code
4. Shows you where query builders and transactions are generated
5. Prints next steps for exploration

### Option B: Manual Setup

```bash
# 1. From project root:
../../bin/fabrica generate

# 2. Ensure database directory exists:
mkdir -p data

# 3. Run server:
export DATABASE_URL="file:./data/demo.db?cache=shared&_fk=1"
go run ./cmd/server/
```

### Files to Explore

- [PATTERNS.md](./PATTERNS.md) - Practical handler patterns and recipes
- [quick-start.sh](./quick-start.sh) - Automated setup script
- Generated files:
  - `internal/storage/ent_queries_generated.go` - Query builders
  - `internal/storage/ent_transactions_generated.go` - Transaction support
  - `cmd/server/*_handlers_generated.go` - Handler integration points

### 4. Test Queries

```bash
# Create servers with labels
curl -X POST http://localhost:8080/api/v1/servers \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "server-prod-1",
      "labels": {"env": "prod", "zone": "us-east-1"}
    },
    "spec": {
      "hostname": "prod-1.example.com",
      "ipAddress": "10.0.1.10"
    }
  }'

# Export resources (offline, no running API required)
go run ./cmd/server export --format yaml --output ./backup

# Import resources
go run ./cmd/server import --input ./backup --mode upsert
```

## Architecture & Patterns

### Query Builders: Efficient Database Queries

The generated storage layer provides type-safe query builders:

```go
// Simple query - returns all servers
servers, err := storage.QueryServers(ctx).All(ctx)

// Query with ordering
servers, err := storage.QueryServers(ctx).
    Order(ent.Asc("created_at")).
    All(ctx)

// Query with limit/offset (pagination)
servers, err := storage.QueryServers(ctx).
    Limit(10).
    Offset(20).
    All(ctx)
```

**Key benefit:** These use Ent's type-safe query builders, producing efficient SQL instead of loading everything into memory.

### Label Selectors: Production Query Patterns

Filter resources by labels without loading all data:

```go
// Find production servers in us-east-1
prodServers, err := storage.ListServersByLabels(ctx, map[string]string{
    "env":  "prod",
    "zone": "us-east-1",
})
// Generated SQL: WHERE kind='Server' AND label.key='env' AND label.value='prod'
//                AND label.key='zone' AND label.value='us-east-1'
```

**Real-world use case:**
```go
// API handler for filtering
func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
    labels := r.URL.Query()
    servers, err := storage.ListServersByLabels(r.Context(), labels)
    // Returns only matching servers, not all 10K+ servers
}
```

### Transactions: Atomic Multi-Resource Operations

Create related resources atomically—either both succeed or both fail:

```go
// Create a server with a default configuration resource
err := storage.WithTx(ctx, func(tx *ent.Tx) error {
    // 1. Create server
    server := &Server{
        Metadata: Metadata{UID: "srv-123", Name: "server-1"},
        Spec: ServerSpec{Hostname: "srv1.example.com"},
    }
    if err := createServerInTx(ctx, tx, server); err != nil {
        return err  // Rollback happens automatically
    }

    // 2. Create associated config (if server creation fails, config never created)
    config := &ServerConfig{
        Metadata: Metadata{UID: "cfg-123", Name: "config-1"},
        Spec: ServerConfigSpec{ServerUID: "srv-123"},
    }
    if err := createConfigInTx(ctx, tx, config); err != nil {
        return err  // Rollback both server and config
    }

    return nil  // Commit both
})
```

**Why this matters:** Without transactions, if the config creation failed halfway through, you'd have an orphaned server. Transactions guarantee consistency.

### Export/Import: Backup & Migration

Save resources to human-readable files for backup, version control, or migration:

```bash
# Export all resources
go run ./cmd/server export --format yaml --output ./backup/

# Export specific resource types
go run ./cmd/server export --kinds Server,ServerConfig --output ./prod-resources/

# Export with label filtering (future: k8s-style selectors)
go run ./cmd/server export --label-selector env=prod --output ./prod-backup/
```

**Output structure:**
```
backup/
├── servers/
│   ├── server-001.yaml
│   ├── server-002.yaml
│   └── server-003.yaml
└── serverconfigs/
    ├── config-001.yaml
    └── config-002.yaml
```

**Import with modes:**
```bash
# Upsert mode (create new, update existing)
go run ./cmd/server import --input ./backup/

# Replace mode (delete all, then import)
go run ./cmd/server import --input ./backup/ --mode replace

# Skip existing (only create new)
go run ./cmd/server import --input ./backup/ --mode skip

# Preview without applying
go run ./cmd/server import --input ./backup/ --dry-run
```

## Code Organization

```
09-ent-advanced/
├── README.md                    # This file
├── go.mod                       # Go module definition
├── apis.yaml                    # API group & version config
├── apis/
│   └── infra.example.com/
│       └── v1/
│           ├── server_types.go          # Server resource definition
│           ├── serverconfig_types.go    # ServerConfig resource definition
│           └── register_generated.go    # Generated resource registration
├── internal/
│   └── storage/
│       ├── ent_queries_generated.go     # Generated query builders
│       ├── ent_transactions_generated.go # Generated transaction helpers
│       ├── ent/
│       │   ├── schema/
│       │   │   ├── resource.go         # Generic resource schema
│       │   │   ├── label.go            # Labels schema
│       │   │   └── annotation.go       # Annotations schema
│       │   └── *.go                    # Ent generated code
│       └── ent_adapter.go              # Fabrica ↔ Ent adapter
└── cmd/
    └── server/
        ├── main.go                     # Server entry point
        ├── routes_generated.go         # Generated REST routes
        ├── server_handlers_generated.go # Generated handlers
        └── serverconfig_handlers_generated.go
```

## Generated Functions

When you run `fabrica generate` with Ent storage, you get:

### Query Functions (internal/storage/ent_queries_generated.go)

```go
// Query all servers
func QueryServers(ctx context.Context) *ent.ResourceQuery

// Query servers by label filters
func ListServersByLabels(ctx context.Context, labels map[string]string) (
    []*server.Server, error)

// Get single server
func GetServerByUID(ctx context.Context, uid string) (
    *server.Server, error)
```

### Transaction Functions (internal/storage/ent_transactions_generated.go)

```go
// Execute operations atomically
func WithTx(ctx context.Context, fn func(*ent.Tx) error) error
```

## Testing the Patterns

### Pattern 1: Label-Based Filtering

```bash
# Create servers with different labels
for i in 1 2 3; do
  curl -X POST http://localhost:8080/api/v1/servers \
    -H "Content-Type: application/json" \
    -d "{
      \"metadata\": {
        \"name\": \"prod-server-$i\",
        \"labels\": {\"env\": \"prod\", \"tier\": \"web\"}
      },
      \"spec\": {\"hostname\": \"prod-$i.example.com\"}
    }"
done

# Query production servers (future: expose via query parameter)
# Generated code: ListServersByLabels(ctx, {"env": "prod"})
```

### Pattern 2: Transactions

Create a server handler that uses transactions for consistency:

```go
// See PATTERNS.md for a full implementation
func (h *Handler) CreateServerWithDefaults(w http.ResponseWriter, r *http.Request) {
    // Parse request...

    // Atomically create server + default config
    err := storage.WithTx(r.Context(), func(tx *ent.Tx) error {
        // Create server in transaction
        server := &Server{...}
        // Use tx instead of entClient
        return nil
    })

    // If any step fails, entire operation rolls back
}
```

### Pattern 3: Export & Import

Generated servers include export/import commands for backup and migration:

```bash
# Export all resources to YAML
go run ./cmd/server export --format yaml --output ./backup

# Export specific resource types
go run ./cmd/server export --kinds Server --output ./server-backup

# Import from backup
go run ./cmd/server import --input ./backup

# Dry run to preview changes
go run ./cmd/server import --input ./backup --dry-run

# Replace mode (delete all resources first)
go run ./cmd/server import --input ./backup --mode replace
```

**Use cases:**
- Regular backups for disaster recovery
- Migrating data between dev/staging/prod environments
- Version controlling resource definitions in Git
- Seeding test data or initial configurations

See [Example 10 - Export/Import](../10-export-import/) for complete workflows.

## Performance Characteristics

| Operation | Performance | Notes |
|-----------|-------------|-------|
| `QueryServers()` | O(n) SQL query | Single DB roundtrip |
| `ListServersByLabels()` | O(n) SQL WHERE | Labels indexed, single query |
| `WithTx()` | ACID guarantees | Multiple operations batched |
| `Export` | O(n) iteration | Stream to files |
| `Import` | O(n) batch upsert | Transaction per batch |

## Advanced Patterns (Recipes)

### Bulk Update with Transaction

```go
func BulkUpdateServerStatus(ctx context.Context, serverUIDs []string, newStatus string) error {
    return storage.WithTx(ctx, func(tx *ent.Tx) error {
        for _, uid := range serverUIDs {
            // Update each server's status atomically
            // All succeed or all fail
        }
        return nil
    })
}
```

### Migration Script

```bash
#!/bin/bash
# Migrate from file storage to PostgreSQL

# Export from old system (file storage)
OLD_API_URL="http://old-api:8080"
curl $OLD_API_URL/api/v1/export > ./migration.yaml

# Start new PostgreSQL-backed API
export DATABASE_URL="postgres://user:pass@pg.example.com/api"
./new-api &

# Wait for startup
sleep 5

# Import into new system
go run ./cmd/server import --input ./migration.yaml --mode replace

echo "Migration complete!"
```

### Label Selector Helper

For more complex filtering, build on generated `QueryResourcesByLabels`:

```go
// Custom function: query servers by multiple conditions
func FindActiveProductionServers(ctx context.Context) ([]*Server, error) {
    // Use generated function
    servers, err := storage.ListServersByLabels(ctx, map[string]string{
        "env":    "prod",
        "status": "active",
    })
    if err != nil {
        return nil, err
    }

    // Further filter in application layer if needed
    var active []*Server
    for _, s := range servers {
        if s.Status.IsActive() {
            active = append(active, s)
        }
    }
    return active, nil
}
```

## Next Steps

1. **Add reconciliation** - See [Example 04](../04-rack-reconciliation/) for reconciler patterns
2. **Add events** - See [Example 05](../05-cloud-events/) for event publishing
3. **Add versioning** - See [Example 08](../08-api-versioning/) for API versioning
4. **Production deployment** - Switch to PostgreSQL, set up migrations, enable metrics

## Common Issues & Troubleshooting

**Issue:** "ent client not initialized"
- **Cause:** Storage backend not set up in main.go
- **Fix:** Call `storage.SetEntClient(entClient)` after creating Ent client

**Issue:** Label queries return empty results
- **Cause:** Labels stored during create, but querying before creation completes
- **Fix:** Ensure transaction is committed before querying

**Issue:** Import fails with "resource already exists"
- **Cause:** Using skip-existing mode but resources exist
- **Fix:** Use upsert mode to update existing, or replace mode to start fresh

## References

- [Ent Documentation](https://entgo.io/docs/getting-started)
- [Storage Guide](../../docs/guides/storage-ent.md)
- [Fabrica Reference](../../docs/reference/architecture.md)
