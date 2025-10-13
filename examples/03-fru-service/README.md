<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 3: FRU Service with SQLite, Casbin, and TokenSmith

**Time to complete:** ~30 minutes
**Difficulty:** Advanced
**Prerequisites:** Go 1.23+, fabrica CLI installed, SQLite3

## What You'll Build

A Field Replaceable Unit (FRU) inventory service with:
- **SQLite Database** - Persistent storage using Ent ORM
- **Generated Middleware** - Validation, conditional requests, and versioning
- **Status Updates** - Track FRU lifecycle and health status
- **Type-Safe API** - Full CRUD operations with OpenAPI spec

This example demonstrates how fabrica generates a complete service with Ent storage backend.

## Architecture Overview

```
FRU Service
├── SQLite Database (Ent ORM)
│   └── FRU resources
├── Generated Middleware
│   ├── Validation (strict mode)
│   ├── Conditional requests (ETags)
│   └── API versioning
└── REST API
    └── CRUD operations for FRUs
```

**Note:** This example focuses on the core Fabrica features. Casbin and TokenSmith integration mentioned in testing sections are advanced features that require additional manual setup beyond the generated code

## Step-by-Step Guide

### Step 1: Initialize with Advanced Features

```bash
# Create project with SQLite storage and authentication
fabrica init fru-service \
  --module github.com/example/fru-service \
  --storage-type ent \
  --db sqlite \
  --auth \
  --validation-mode strict

cd fru-service
```

**What gets created:**
```
fru-service/
├── .fabrica.yaml                    # Configuration with ent storage + auth
├── cmd/
│   └── server/
│       └── main.go                  # Server with Ent setup
├── internal/
│   └── storage/
│       └── ent/                     # Ent schema directory
└── pkg/resources/                   # Empty (for FRU resource)
```

### Step 2: Add the FRU Resource

```bash
fabrica add resource FRU
```

### Step 3: Copy the FRU Resource Definition

Copy the FRU resource from the example:

```bash
cp -r ../../fabrica/examples/03-fru-service/pkg/resources/fru pkg/resources/
```

Or create your own `pkg/resources/fru/fru.go` with this structure:

```go
// FRUSpec defines the desired state of FRU
type FRUSpec struct {
    // FRU identification
    FRUType      string `json:"fruType"`      // e.g., "CPU", "Memory", "Storage"
    SerialNumber string `json:"serialNumber"`
    PartNumber   string `json:"partNumber"`
    Manufacturer string `json:"manufacturer"`
    Model        string `json:"model"`

    // Location information
    Location FRULocation `json:"location"`

    // Relationships
    ParentUID    string   `json:"parentUID,omitempty"`
    ChildrenUIDs []string `json:"childrenUIDs,omitempty"`

    // Redfish path for management
    RedfishPath string `json:"redfishPath,omitempty"`
}

// FRUStatus defines the observed state of FRU
type FRUStatus struct {
    Health      string             `json:"health"`      // "OK", "Warning", "Critical", "Unknown"
    State       string             `json:"state"`       // "Present", "Absent", "Disabled", "Unknown"
    Functional  string             `json:"functional"`  // "Enabled", "Disabled", "Unknown"
    LastSeen    string             `json:"lastSeen,omitempty"`
    LastScanned string             `json:"lastScanned,omitempty"`
    Errors      []string           `json:"errors,omitempty"`
    Temperature float64            `json:"temperature,omitempty"`
    Power       float64            `json:"power,omitempty"`
    Metrics     map[string]float64 `json:"metrics,omitempty"`
}
```

The FRU resource tracks hardware inventory with detailed location and status information.

### Step 4: Add Replace Directive for Local Development

```bash
echo -e "\nreplace github.com/alexlovelltroy/fabrica => /Users/alt/Development/fabrica/fabrica" >> go.mod
```

### Step 5: Generate All Code

```bash
fabrica generate
```

### Step 5a: Generate Ent Client Code

After running `fabrica generate`, you must generate the Ent client code:

```bash
fabrica ent generate
```

This step is **required** when using Ent storage. It generates the Ent ORM client code based on the schemas created in Step 6.

**What gets generated:**
```
fru-service/
├── cmd/server/
│   ├── fru_handlers_generated.go     # CRUD handlers with auth checks
│   ├── models_generated.go           # Request/response models
│   ├── routes_generated.go           # Routes with auth middleware
│   ├── openapi_generated.go          # OpenAPI spec
│   └── policy_handlers.go            # Casbin policy endpoints
├── internal/
│   ├── middleware/                   # Core middleware
│   │   ├── validation_middleware_generated.go
│   │   ├── conditional_middleware_generated.go
│   │   └── versioning_middleware_generated.go
│   └── storage/
│       ├── ent/                      # Generated Ent code
│       │   ├── schema/               # Resource schema
│       │   └── ...                   # Ent client code
│       ├── ent_adapter.go            # Ent-to-Storage adapter
│       └── storage_generated.go      # Storage functions
└── pkg/client/
    └── ...                           # Client library
```

### Step 6: Uncomment Server Setup in main.go

The generated `cmd/server/main.go` has storage and routing code commented out. You need to uncomment these sections to activate them.

#### Step 7a: Uncomment Storage Imports

Find these lines near the top of `cmd/server/main.go` (around line 23-28):

```go
// TODO: Uncomment after running 'fabrica generate'
// "github.com/example/fru-service/internal/storage"

// TODO: Uncomment after running 'fabrica generate --storage'
// "github.com/example/fru-service/internal/storage/ent"
// "github.com/example/fru-service/internal/storage/ent/migrate"
```

Uncomment them:

```go
"github.com/example/fru-service/internal/storage"
"github.com/example/fru-service/internal/storage/ent"
"github.com/example/fru-service/internal/storage/ent/migrate"
```

#### Step 7b: Uncomment Storage Initialization

Find the storage initialization section (around line 207-227):

```go
// TODO: Connect to database after running 'fabrica generate --storage'
// client, err := ent.Open("sqlite3", config.DatabaseURL)
// if err != nil {
//     return fmt.Errorf("failed opening connection to sqlite: %w", err)
// }
// defer client.Close()

// Run auto-migration
// ctx := context.Background()
// if err := client.Schema.Create(
//     ctx,
//     migrate.WithDropIndex(true),
//     migrate.WithDropColumn(true),
// ); err != nil {
//     return fmt.Errorf("failed creating schema resources: %w", err)
// }
// log.Println("Database schema migrated successfully")

// Set Ent client for storage operations
// TODO: Uncomment after running 'fabrica generate --storage'
// storage.SetEntClient(client)
```

Uncomment all the storage code:

```go
client, err := ent.Open("sqlite3", config.DatabaseURL)
if err != nil {
    return fmt.Errorf("failed opening connection to sqlite: %w", err)
}
defer client.Close()

// Run auto-migration
ctx := context.Background()
if err := client.Schema.Create(
    ctx,
    migrate.WithDropIndex(true),
    migrate.WithDropColumn(true),
); err != nil {
    return fmt.Errorf("failed creating schema resources: %w", err)
}
log.Println("Database schema migrated successfully")

// Set Ent client for storage operations
storage.SetEntClient(client)
```

#### Step 7c: Uncomment Route Registration

Find the route registration calls (around lines 261 and 271). Change:

```go
// TODO: RegisterGeneratedRoutes(r) - Run 'fabrica generate' to create routes
```

To:

```go
RegisterGeneratedRoutes(r)
```

There are two occurrences - one in the auth-enabled block and one in the auth-disabled block. Uncomment both.

**Note:** The generated code does NOT include Casbin or TokenSmith integration by default. Those are mentioned in this README as advanced features but require manual implementation. For now, we'll run without them to get a working server

### Step 7: Install Dependencies

```bash
go mod tidy
```

Required dependencies:
- `entgo.io/ent` - ORM framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/casbin/casbin/v2` - Authorization
- `github.com/OpenCHAMI/tokensmith/middleware` - JWT authentication

### Step 8: Build and Run

```bash
# Build server
go build -o fru-server ./cmd/server

# Run server
./fru-server serve
```

Expected output:
```
2025/10/10 12:00:00 Starting fru-service server...
2025/10/10 12:00:00 Database schema migrated successfully
2025/10/10 12:00:00 Server starting on 0.0.0.0:8080
2025/10/10 12:00:00 Storage: sqlite database
2025/10/10 12:00:00 Authentication: enabled
```

### Step 9: Build Client CLI

```bash
fabrica generate --client
go build -o fru-cli ./cmd/client
```

## Testing the Service

### 1. Create an FRU

```bash
# Use curl to create an FRU
curl -X POST http://localhost:8080/frus \
  -H "Content-Type: application/json" \
  -d '{
  "name": "cpu-001",
  "fruType": "CPU",
  "serialNumber": "CPU12345",
  "partNumber": "XEON-5678",
  "manufacturer": "Intel",
  "model": "Xeon Gold 6248R",
  "location": {
    "rack": "R42",
    "chassis": "C1",
    "slot": "U10",
    "socket": "CPU0"
  },
  "redfishPath": "/redfish/v1/Systems/node-001/Processors/CPU0"
}'
```

Expected response (201 Created):
```json
{
  "apiVersion": "v1",
  "kind": "FRU",
  "metadata": {
    "uid": "fru-a1b2c3d4",
    "createdAt": "2025-10-10T12:05:00Z",
    "updatedAt": "2025-10-10T12:05:00Z"
  },
  "spec": {
    "fruType": "CPU",
    "serialNumber": "CPU12345",
    "partNumber": "XEON-5678",
    "manufacturer": "Intel",
    "model": "Xeon Gold 6248R",
    "location": {
      "rack": "R42",
      "chassis": "C1",
      "slot": "U10",
      "socket": "CPU0"
    },
    "redfishPath": "/redfish/v1/Systems/node-001/Processors/CPU0"
  },
  "status": {
    "health": "OK",
    "state": "Present",
    "functional": "Enabled",
    "lastSeen": "2025-10-10T12:05:00Z"
  }
}
```

Save the UID from the response for later steps.

### 2. List All FRUs

```bash
curl http://localhost:8080/frus
```

### 3. Get Specific FRU

```bash
# Replace {uid} with the actual FRU UID from create response
curl http://localhost:8080/frus/{uid}
```

### 4. Update FRU Status

This demonstrates updating the status of an existing FRU:

```bash
# Get the FRU UID from the create response
FRU_UID="fru-a1b2c3d4"

# Update to reflect maintenance state
curl -X PUT http://localhost:8080/frus/$FRU_UID \
  -H "Content-Type: application/json" \
  -d '{
  "name": "cpu-001",
  "fruType": "CPU",
  "serialNumber": "CPU12345",
  "partNumber": "XEON-5678",
  "manufacturer": "Intel",
  "model": "Xeon Gold 6248R",
  "location": {
    "rack": "R42",
    "chassis": "C1",
    "slot": "U10",
    "socket": "CPU0"
  },
  "redfishPath": "/redfish/v1/Systems/node-001/Processors/CPU0"
}'
```

**Status Update Pattern:**

To update only the status (common for monitoring systems), you can use PATCH:

```bash
# PATCH request to update just the status (if PATCH is implemented)
curl -X PATCH http://localhost:8080/frus/$FRU_UID \
  -H "Content-Type: application/json" \
  -d '{
    "status": {
      "health": "Critical",
      "state": "Present",
      "functional": "Enabled",
      "temperature": 85.0,
      "errors": ["Critical temperature threshold exceeded"],
      "lastSeen": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }
  }'
```

### 5. Delete an FRU

```bash
curl -X DELETE http://localhost:8080/frus/$FRU_UID
```

## Advanced Features Demonstrated

### 1. SQLite with Ent ORM

The generated code uses Ent for database operations:

```go
// From internal/storage/ent_adapter.go
func LoadAllFRUs(ctx context.Context) ([]*fru.FRU, error) {
    resources, err := entClient.Resource.
        Query().
        Where(resource.TypeEQ("FRU")).
        All(ctx)
    // ... unmarshal into FRU objects
}
```

Benefits:
- Type-safe database queries
- Automatic migrations
- Relationship management
- Transaction support

### 2. Status Lifecycle Management

FRUs track detailed status information:

```go
// Status fields track operational state
type FRUStatus struct {
    Health      string             // "OK", "Warning", "Critical", "Unknown"
    State       string             // "Present", "Absent", "Disabled", "Unknown"
    Functional  string             // "Enabled", "Disabled", "Unknown"
    LastSeen    string             // Last time FRU was detected
    LastScanned string             // Last inventory scan timestamp
    Errors      []string           // Error conditions
    Temperature float64            // Temperature in Celsius
    Power       float64            // Power consumption in watts
    Metrics     map[string]float64 // Custom metrics
}
```

## Database Management

### View Database Contents

```bash
# Connect to SQLite database
sqlite3 fru.db

# List all tables
.tables

# View FRU resources
SELECT * FROM resources WHERE type = 'FRU';

# View Casbin policies (if using Ent adapter)
SELECT * FROM casbin_rule;

# Exit
.quit
```

### Backup Database

```bash
# Backup
sqlite3 fru.db ".backup fru-backup.db"

# Restore
sqlite3 fru.db ".restore fru-backup.db"
```

## Troubleshooting

### Issue: "failed to open database: unable to open database file"

**Cause:** SQLite file path or permissions issue
**Fix:** Ensure the directory is writable:
```bash
mkdir -p data
./fru-server --data-dir ./data
```

### Issue: "failed to load policies: file does not exist"

**Cause:** Casbin policy files not found
**Fix:** Ensure `policies/` directory exists with `model.conf` and `policy.csv`:
```bash
ls -la policies/
```

### Issue: "Connection refused"

**Cause:** Server is not running
**Fix:** Start the server with `./fru-server serve`

### Issue: "404 Not Found"

**Cause:** Routes not registered or wrong URL
**Fix:** Ensure you uncommented `RegisterGeneratedRoutes(r)` in Step 6c

## Configuration Reference

### .fabrica.yaml

```yaml
project:
  name: fru-service
  module: github.com/example/fru-service

features:
  validation:
    enabled: true
    mode: strict           # Reject invalid requests

  conditional:
    enabled: true
    etag_algorithm: sha256 # For optimistic locking

  versioning:
    enabled: true
    strategy: header       # Version via Accept header

  events:
    enabled: false
    bus_type: memory

  auth:
    enabled: true          # Enable Casbin authorization
    provider: casbin

  storage:
    enabled: true
    type: ent              # Use Ent ORM
    db_driver: sqlite      # SQLite database

generation:
  handlers: true
  storage: true
  client: true
  openapi: true
```

## Adding Authentication and Authorization (Advanced)

The basic example above works without authentication. To add Casbin and TokenSmith:

### Option 1: Add Casbin for Authorization

1. Create policies directory and files (see original Step 4 above - Casbin setup)
2. Add Casbin imports and initialization in main.go
3. Create middleware to check policies before handler execution
4. See [docs/policy-casbin.md](../../docs/policy-casbin.md) for details

### Option 2: Add TokenSmith for Authentication

1. Install TokenSmith middleware: `go get github.com/OpenCHAMI/tokensmith/middleware`
2. Configure JWKS endpoint and issuer
3. Add middleware to validate JWT tokens
4. See [TokenSmith documentation](https://github.com/OpenCHAMI/tokensmith/tree/main/middleware)

These features require manual integration and are beyond the scope of this basic example.

## Next Steps

- **Add Events:** Enable CloudEvents with `--events` flag during init
- **Add Versioning:** Experiment with different `--version-strategy` options
- **Custom Validation:** Modify validation rules in generated middleware
- **API Gateway:** Deploy behind Kong or similar gateway for rate limiting
- **High Availability:** Switch to PostgreSQL for production deployments
- **Monitoring:** Add Prometheus metrics with `--metrics` flag

## Production Checklist

- [ ] Switch to PostgreSQL or MySQL for production
- [ ] Add authentication (TokenSmith or similar)
- [ ] Add authorization (Casbin or similar)
- [ ] Enable HTTPS/TLS
- [ ] Set up database backups
- [ ] Configure log aggregation
- [ ] Add health check endpoints
- [ ] Set up monitoring and alerts
- [ ] Document your API with generated OpenAPI spec
- [ ] Load test with expected traffic patterns

## Summary

This example demonstrates how fabrica generates complete services with:
- ✅ **Persistent Storage** - SQLite/Ent with automatic migrations
- ✅ **Generated Middleware** - Validation, conditional requests, versioning
- ✅ **Status Management** - Track FRU lifecycle and health
- ✅ **Type Safety** - Compile-time validation throughout
- ✅ **REST API** - Full CRUD operations with OpenAPI spec
- ✅ **Client Library** - Generated CLI and programmatic client
- ✅ **Best Practices** - Generated code follows Go idioms

All generated from a simple resource definition! Add authentication and authorization as needed for your use case.
