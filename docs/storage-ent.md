<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Ent Storage Backend

This guide covers using [Ent](https://entgo.io) as the storage backend for Fabrica-generated microservice APIs.

## Overview

Ent is a powerful entity framework for Go that provides:

- **Type-safe database operations** - Compile-time safety for all queries
- **Automatic migrations** - Schema changes handled automatically
- **Multiple databases** - PostgreSQL, MySQL, and SQLite support
- **Complex queries** - Fluent API for joins, aggregations, and filtering
- **Transactions** - Built-in transaction support
- **Hooks** - Lifecycle hooks for custom logic

## When to Use Ent Storage

**Use Ent when:**
- You need production-grade database storage
- You require complex queries (label selectors, filtering)
- You need transactions for data consistency
- You want automatic schema migrations
- Horizontal scaling is important

**Use File Storage when:**
- Prototyping or development
- Simple applications with low traffic
- No external dependencies desired
- Embedded/edge deployment scenarios

## Quick Start

### Complete Workflow Example

Here's the complete workflow to create and run an Ent-backed Fabrica API:

```bash
# 1. Create a new project with Ent storage
fabrica init my-api --storage=ent --db=postgres
cd my-api

# 2. Add your resources
fabrica add resource Device

# 3. Generate Fabrica code (handlers, storage, etc.)
fabrica generate

# 4. Generate Ent client code from schemas
# This reads internal/storage/ent/schema/*.go
# and generates type-safe database client code
go generate ./internal/storage
# Or use: fabrica ent generate

# 5. Set up your database connection
export DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
# For SQLite development: export DATABASE_URL="file:test.db?cache=shared&_fk=1"

# 6. Build and run (migrations run automatically on startup)
go build -o api ./cmd/server
./api

# 7. Test your API
curl http://localhost:8080/api/v1/devices
```

### What Gets Generated

When you run `fabrica init --storage=ent`, the following files are created:

| File | Purpose |
|------|---------|
| `internal/storage/ent/schema/*.go` | Ent schema definitions (Resource, Label, Annotation) |
| `internal/storage/generate.go` | Contains `//go:generate` directive for Ent code generation |
| `internal/storage/ent_adapter.go` | Converts between Fabrica resources and Ent entities |
| `internal/storage/storage_ent.go` | Ent-backed storage implementation |
| `cmd/server/main.go` | Includes database connection and auto-migration |

**Note:** After running `go generate ./internal/storage`, Ent will create `internal/storage/ent/*.go` files with the generated client code.

### Database Connection Strings

```bash
# PostgreSQL
export DATABASE_URL="postgres://user:pass@localhost/mydb?sslmode=disable"

# MySQL
export DATABASE_URL="user:pass@tcp(localhost:3306)/mydb?parseTime=true"

# SQLite (development/testing)
export DATABASE_URL="file:./data.db?cache=shared&_fk=1"
```

## Architecture

### Hybrid Storage Approach

Fabrica maintains its Kubernetes-style `Resource{Spec, Status}` pattern while using Ent for persistence:

```
┌─────────────────────────────────────────────┐
│         HTTP Handler (Generated)            │
│  1. Decode JSON → Fabrica Resource struct   │
└─────────────────┬───────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────┐
│  Layer 2: Fabrica Struct Tag Validation     │
│  - Validates Spec/Status structure          │
│  - K8s validators (k8sname, labels, etc.)   │
└─────────────────┬───────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────┐
│  Layer 3: Custom Business Validation        │
│  - Cross-field validation                   │
│  - Database lookups                         │
└─────────────────┬───────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────┐
│      Ent Adapter (Generated)                │
│  - Marshal Spec/Status to JSON              │
│  - Convert Fabrica Resource → Ent Entity    │
└─────────────────┬───────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────┐
│  Layer 1: Ent Schema Validation             │
│  - Field constraints                        │
│  - Unique constraints                       │
└─────────────────┬───────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────────┐
│          Database                           │
└─────────────────────────────────────────────┘
```

### Database Schema

Ent uses three tables to store Fabrica resources:

**resources table:**
```sql
CREATE TABLE resources (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(253) NOT NULL,
    api_version VARCHAR(50) DEFAULT 'v1',
    kind VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    spec JSONB NOT NULL,              -- Desired state as JSON
    status JSONB,                      -- Observed state as JSON
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    resource_version VARCHAR(50) DEFAULT '1',
    namespace VARCHAR(253)
);
```

**labels table:**
```sql
CREATE TABLE labels (
    id SERIAL PRIMARY KEY,
    resource_id INTEGER REFERENCES resources(id) ON DELETE CASCADE,
    key VARCHAR(253) NOT NULL,
    value VARCHAR(63)
);
```

**annotations table:**
```sql
CREATE TABLE annotations (
    id SERIAL PRIMARY KEY,
    resource_id INTEGER REFERENCES resources(id) ON DELETE CASCADE,
    key VARCHAR(253) NOT NULL,
    value TEXT
);
```

## Three-Layer Validation

Ent integration maintains Fabrica's comprehensive validation approach:

### Layer 1: Ent Schema Validation (Database Level)

Defined in generated `internal/storage/ent/schema/resource.go`:

```go
field.String("uid").
    Unique().
    Immutable().
    NotEmpty()

field.String("name").
    NotEmpty().
    MaxLen(253)

field.JSON("spec", json.RawMessage{}).
    Validate(func(data json.RawMessage) error {
        if len(data) == 0 {
            return fmt.Errorf("spec cannot be empty")
        }
        return nil
    })
```

**Validates:**
- Field types and constraints
- Uniqueness (UIDs)
- JSON structure
- Database-level rules

**When:** Automatically during `.Save()` / `.Create()`

### Layer 2: Fabrica Struct Tag Validation (Application Level)

In your resource definitions:

```go
type DeviceSpec struct {
    Name      string `json:"name" validate:"required,k8sname"`
    Location  string `json:"location" validate:"required,min=1,max=100"`
    Model     string `json:"model" validate:"required"`
    IPAddress string `json:"ipAddress" validate:"omitempty,ip"`
}
```

In handlers (auto-generated):

```go
// Layer 2: Struct tag validation
if err := validation.ValidateResource(&device); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}
```

**Validates:**
- Spec/Status field constraints
- Kubernetes naming conventions
- Custom validators

**When:** In HTTP handlers before database save

### Layer 3: Custom Business Logic (Business Rules)

Implement `Validate(ctx)` on your resources:

```go
func (d *Device) Validate(ctx context.Context) error {
    // Cross-field validation
    if d.Spec.Location == "production" && d.Spec.Model == "" {
        return fmt.Errorf("model required for production devices")
    }

    // Database lookup validation
    if d.Spec.ParentDeviceID != "" {
        exists, err := deviceExists(ctx, d.Spec.ParentDeviceID)
        if err != nil || !exists {
            return fmt.Errorf("parent device not found")
        }
    }

    // Business rules
    if d.Spec.IPAddress != "" {
        available, _ := isIPAvailable(ctx, d.Spec.IPAddress)
        if !available {
            return fmt.Errorf("IP address already in use")
        }
    }

    return nil
}
```

In handlers:

```go
// Layer 3: Custom validation
if err := validation.ValidateWithContext(r.Context(), &device); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}
```

**Validates:**
- Cross-field rules
- External dependencies
- Complex business logic

**When:** After struct validation, before database save

## Working with Resources

### Creating Resources

```go
device := &device.Device{
    Resource: resource.Resource{
        APIVersion: "v1",
        Kind:       "Device",
    },
    Spec: device.DeviceSpec{
        Name:     "sensor-001",
        Location: "Building A",
        Model:    "TMP-100",
    },
}

device.Metadata.Initialize("sensor-001", uid)
device.SetLabel("environment", "production")

// All three validation layers execute
err := storage.SaveDevice(ctx, device)
```

### Querying Resources

```go
// Load single resource
device, err := storage.LoadDevice(ctx, "dev-abc123")

// Load all devices
devices, err := storage.LoadAllDevices(ctx)
```

### Querying by Labels

For advanced queries, use the Ent client directly:

```go
// In your custom code
devices, err := entClient.Resource.Query().
    Where(resource.KindEQ("Device")).
    Where(resource.HasLabelsWith(
        label.KeyEQ("environment"),
        label.ValueEQ("production"),
    )).
    WithLabels().
    All(ctx)
```

### Updating Resources

```go
device, err := storage.LoadDevice(ctx, uid)
if err != nil {
    return err
}

device.Spec.Location = "Building B"
device.Touch() // Update timestamp

err = storage.SaveDevice(ctx, device)
```

### Deleting Resources

```go
err := storage.DeleteDevice(ctx, uid)
if errors.Is(err, storage.ErrNotFound) {
    // Handle not found
}
```

## Migrations

### Automatic Migrations

Fabrica-generated main.go includes auto-migration:

```go
// Run auto-migration
if err := client.Schema.Create(
    ctx,
    migrate.WithDropIndex(true),
    migrate.WithDropColumn(true),
); err != nil {
    log.Fatalf("failed creating schema: %v", err)
}
```

**Development:** Safe for rapid iteration
**Production:** Use versioned migrations instead

### Manual Migrations

For production, use Ent's migration system:

```bash
# Generate migration files
fabrica ent migrate --dry-run > migrations/001_init.sql

# Review and apply
psql $DATABASE_URL < migrations/001_init.sql
```

## Database Drivers

### PostgreSQL (Recommended for Production)

```bash
fabrica init my-api --storage=ent --db=postgres
```

**go.mod includes:**
```go
require github.com/lib/pq latest
```

**Connection string:**
```bash
export DATABASE_URL="postgres://user:pass@localhost/dbname?sslmode=disable"
```

**Features:**
- JSONB for efficient Spec/Status queries
- Full-text search capabilities
- Mature replication and scaling

### MySQL

```bash
fabrica init my-api --storage=ent --db=mysql
```

**go.mod includes:**
```go
require github.com/go-sql-driver/mysql latest
```

**Connection string:**
```bash
export DATABASE_URL="user:pass@tcp(localhost:3306)/dbname?parseTime=true"
```

### SQLite (Development)

```bash
fabrica init my-api --storage=ent --db=sqlite
```

**go.mod includes:**
```go
require github.com/mattn/go-sqlite3 latest
```

**Connection string:**
```bash
export DATABASE_URL="file:./data.db?cache=shared&_fk=1"
```

**Use for:**
- Local development
- Testing
- Embedded scenarios

## Advanced Topics

### Transactions

For operations requiring atomicity:

```go
// Start transaction
tx, err := entClient.Tx(ctx)
if err != nil {
    return err
}

// Multiple operations
device1, err := tx.Resource.Create()./* ... */.Save(ctx)
if err != nil {
    tx.Rollback()
    return err
}

device2, err := tx.Resource.Create()./* ... */.Save(ctx)
if err != nil {
    tx.Rollback()
    return err
}

// Commit
return tx.Commit()
```

### Custom Queries

Access the Ent client for advanced queries:

```go
// Complex filtering
devices, err := entClient.Resource.Query().
    Where(
        resource.KindEQ("Device"),
        resource.HasLabelsWith(
            label.Or(
                label.ValueEQ("production"),
                label.ValueEQ("staging"),
            ),
        ),
    ).
    Order(ent.Desc(resource.FieldCreatedAt)).
    Limit(10).
    All(ctx)
```

### Aggregations

```go
// Count resources by type
counts, err := entClient.Resource.Query().
    GroupBy(resource.FieldKind).
    Aggregate(ent.Count()).
    Ints(ctx)
```

## Troubleshooting

### "ent schema directory not found" Error

If you see this when running `fabrica ent` commands:

```bash
# Check if you're in an Ent-enabled project
ls internal/storage/ent/schema

# If the directory doesn't exist, your project wasn't initialized with Ent
# Create a new Ent-enabled project:
fabrica init my-new-api --storage=ent
```

### "package ent is not in GOROOT" Error

This means Ent client code hasn't been generated yet:

```bash
# Generate Ent client code
go generate ./internal/storage

# Or use the Fabrica command
fabrica ent generate
```

### Connection Issues

```bash
# Test database connection
psql $DATABASE_URL -c "SELECT 1"

# Verify DATABASE_URL is set
echo $DATABASE_URL

# Check Ent client initialization in logs
tail -f /var/log/myapp.log | grep "ent"
```

### Migration Failures

```bash
# Check current database state
fabrica ent describe

# Drop and recreate (DEVELOPMENT ONLY - destroys all data!)
psql $DATABASE_URL -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
fabrica ent migrate

# For production, use versioned migrations
```

### Performance Issues

**Enable query logging:**
```go
client, err := ent.Open("postgres", dbURL,
    ent.Debug(), // Log all queries
)
```

**Add indexes:**
```go
// In schema definition
func (Resource) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("kind", "created_at"),
        index.Fields("namespace", "name"),
    }
}
```

## Best Practices

1. **Use transactions** for multi-resource operations
2. **Add indexes** for frequently queried fields
3. **Use connection pooling** in production
4. **Monitor query performance** with `ent.Debug()`
5. **Version your migrations** for production deployments
6. **Validate before save** using all three layers
7. **Handle ErrNotFound** explicitly
8. **Use prepared statements** for repeated queries

## Migration from File Storage

To migrate existing file-based projects to Ent:

1. **Backup data:**
   ```bash
   tar -czf backup.tar.gz ./data
   ```

2. **Initialize Ent:**
   ```bash
   fabrica init new-project --storage=ent --db=postgres
   cp -r pkg/resources new-project/pkg/
   ```

3. **Regenerate with Ent:**
   ```bash
   cd new-project
   fabrica generate
   fabrica ent generate
   ```

4. **Migrate data** (write custom script):
   ```go
   // Load from file backend
   devices, _ := fileBackend.LoadAll(ctx, "Device")

   // Save to Ent backend
   for _, device := range devices {
       entBackend.Save(ctx, "Device", device.UID, device)
   }
   ```

## Next Steps

- [Validation Guide](validation.md) - Three-layer validation in depth
- [Storage Guide](storage.md) - Storage abstraction overview
- [Architecture](architecture.md) - System design

## References

- [Ent Documentation](https://entgo.io/docs/getting-started)
- [Ent Schema Guide](https://entgo.io/docs/schema-def)
- [Ent Migrations](https://entgo.io/docs/migrate)
- [PostgreSQL JSONB](https://www.postgresql.org/docs/current/datatype-json.html)
