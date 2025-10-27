<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 3: FRU Service with SQLite and Ent Storage

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

**Note:** This example focuses on the core Fabrica features including resource management, storage, and API generation

## Step-by-Step Guide

### Step 1: Initialize with Advanced Features

```bash
# Create project with SQLite storage
fabrica init fru-service \
  --module github.com/example/fru-service \
  --storage-type ent \
  --db sqlite \
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

// FRULocation defines where the FRU is located
type FRULocation struct { //nolint:revive
	BMCUID   string `json:"bmcUID,omitempty"`  // BMC managing this FRU
	NodeUID  string `json:"nodeUID,omitempty"` // Node containing this FRU
	Rack     string `json:"rack,omitempty"`
	Chassis  string `json:"chassis,omitempty"`
	Slot     string `json:"slot,omitempty"`
	Bay      string `json:"bay,omitempty"`
	Position string `json:"position,omitempty"`
	Socket   string `json:"socket,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Port     string `json:"port,omitempty"`
}

// FRUStatus defines the observed state of FRU
type FRUStatus struct {
    Health      string               `json:"health"`      // "OK", "Warning", "Critical", "Unknown"
    State       string               `json:"state"`       // "Present", "Absent", "Disabled", "Unknown"
    Functional  string               `json:"functional"`  // "Enabled", "Disabled", "Unknown"
    LastSeen    string               `json:"lastSeen,omitempty"`
    LastScanned string               `json:"lastScanned,omitempty"`
    Errors      []string             `json:"errors,omitempty"`
    Temperature float64              `json:"temperature,omitempty"`
    Power       float64              `json:"power,omitempty"`
    Metrics     map[string]float64   `json:"metrics,omitempty"`
    Conditions  []resource.Condition `json:"conditions,omitempty"`
}
```

The FRU resource tracks hardware inventory with detailed location and status information.


### Step 4: Generate All Code

```bash
fabrica generate
```

**Note:** Ent client code generation runs automatically when Ent storage is detected. The `fabrica ent generate` command is deprecated but still available for backward compatibility.

### Step 5: Update Dependencies

After code generation is complete, update your Go module dependencies:

```bash
go mod tidy
```

This resolves all the new imports that were added by the code generators.

**What gets generated:**
```
fru-service/
├── cmd/server/
│   ├── fru_handlers_generated.go     # CRUD handlers with auth checks
│   ├── models_generated.go           # Request/response models
│   ├── routes_generated.go           # Routes with auth middleware
│   ├── openapi_generated.go          # OpenAPI spec
│   └── *_handlers_generated.go       # Generated CRUD handlers
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

### Step 6: Verify Dependencies

The required dependencies should now be installed:
- `entgo.io/ent` - ORM framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/alexlovelltroy/fabrica/pkg/*` - Fabrica framework packages

### Step 7: Build and Run

```bash
# Create directory for database
mkdir -p data

# Build server
go build -o fru-server ./cmd/server

# Run server with SQLite foreign keys enabled
./fru-server serve --database-url "file:data/fru.db?_fk=1"
```

Expected output:
```
2025/10/10 12:00:00 Starting fru-service server...
2025/10/10 12:00:00 Database schema migrated successfully
2025/10/10 12:00:00 Server starting on 0.0.0.0:8080
2025/10/10 12:00:00 Storage: sqlite database
2025/10/10 12:00:00 Authentication: enabled
```

### Step 8: Build Client CLI

```bash
fabrica generate --client
go build -o fru-cli ./cmd/client
```

## Testing the Service

### 1. Create an FRU

```bash
# Create an FRU using the generated CLI client with --spec flag
./fru-cli fru create --spec '{
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

Alternatively, create from a JSON file:

```bash
# Create fru-cpu.json file
cat > fru-cpu.json <<EOF
{
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
}
EOF

# Create FRU from file using stdin
cat fru-cpu.json | ./fru-cli fru create
```

Expected output:
```
Created FRU: fru-a1b2c3d4
Name: cpu-001
Type: CPU
Serial: CPU12345
Status: Present/Enabled/OK
```

Save the UID from the response for later steps.

### 2. List All FRUs

```bash
# List all FRUs in table format (default)
./fru-cli fru list

# List in JSON format for processing
./fru-cli fru list --output json

# List in YAML format
./fru-cli fru list --output yaml
```

Example output:
```
NAME       TYPE     SERIAL      MANUFACTURER  STATUS    LOCATION
cpu-001    CPU      CPU12345    Intel         OK        R42/C1/U10/CPU0
memory-001 Memory   MEM12345    Samsung       OK        R42/C1/U10/DIMM_A1
```

### 3. Get Specific FRU

```bash
# Get FRU by UID
./fru-cli fru get fru-a1b2c3d4

# Get with detailed output in JSON
./fru-cli fru get fru-a1b2c3d4 --output json
```

**Note:** The CLI supports getting resources by UID only.

Example output:
```
FRU Details:
  UID: fru-a1b2c3d4
  Name: cpu-001
  Type: CPU
  Serial Number: CPU12345
  Part Number: XEON-5678
  Manufacturer: Intel
  Model: Xeon Gold 6248R

Location:
  Rack: R42
  Chassis: C1
  Slot: U10
  Socket: CPU0

Status:
  Health: OK
  State: Present
  Functional: Enabled
  Temperature: 65.0°C
  Last Seen: 2025-10-10T12:05:00Z
```

### 4. Update FRU Specification

This demonstrates updating the specification of an existing FRU. Remember: only spec fields can be modified by users.

```bash
# Get the FRU UID from the create response
FRU_UID="fru-a1b2c3d4"

# Update FRU specification using update command
./fru-cli fru update $FRU_UID --spec '{
  "manufacturer": "Intel Corporation",
  "model": "Xeon Gold 6248R v2",
  "partNumber": "XEON-5678-V2",
  "location": {
    "rack": "R42",
    "chassis": "C1",
    "slot": "U10",
    "socket": "CPU0",
    "position": "Primary"
  },
  "properties": {
    "cores": "24",
    "threads": "48",
    "baseFreq": "3.0GHz",
    "maxFreq": "4.0GHz"
  }
}'

# Update with spec file using stdin
cat > fru-spec-update.json <<EOF
{
  "manufacturer": "Intel Corporation",
  "model": "Xeon Gold 6248R v2",
  "partNumber": "XEON-5678-V2",
  "properties": {
    "warranty": "3years",
    "purchaseDate": "2025-01-15",
    "vendor": "Dell"
  }
}
EOF

cat fru-spec-update.json | ./fru-cli fru update $FRU_UID
```

**Spec vs Status:**

- **Spec fields** (user-modifiable): Hardware specifications, location, properties, relationships
- **Status fields** (API-managed): Health, operational state, temperature, errors, conditions

```bash
# ✅ Correct: Update spec fields
./fru-cli fru update $FRU_UID --spec '{
  "manufacturer": "AMD",
  "model": "EPYC 7763",
  "properties": {"cores": "64"}
}'

# ❌ Incorrect: Trying to update status (will be ignored by API)
# Status is managed automatically by the system based on:
# - Hardware monitoring
# - Health checks
# - Business logic
# - External integrations
```

### 5. Patch Operations

The patch command allows efficient partial updates to FRU specifications. Only spec fields can be modified - status and metadata are API-managed.

#### JSON Merge Patch (Simple)

```bash
# Update manufacturer and model
./fru-cli fru patch $FRU_UID --spec '{
  "manufacturer": "Samsung",
  "model": "32GB DDR4-3200",
  "properties": {
    "speed": "3200MHz",
    "capacity": "32GB"
  }
}'
```

#### Shorthand Patch (Convenient)

```bash
# Update individual fields using dot notation
./fru-cli fru patch $FRU_UID \
  --set manufacturer=Samsung \
  --set model="32GB DDR4-3200" \
  --set properties.speed=3200MHz \
  --unset properties.oldField
```

#### JSON Patch (Most Powerful)

```bash
# Complex operations with JSON Patch
./fru-cli fru patch $FRU_UID --json-patch '[
  {"op": "replace", "path": "/manufacturer", "value": "Samsung"},
  {"op": "add", "path": "/properties/tested", "value": true},
  {"op": "remove", "path": "/properties/legacy"}
]'
```

#### Status vs Spec

**Important Distinction:**
- **Spec fields** (user-modifiable): `fruType`, `serialNumber`, `manufacturer`, `model`, `location`, `properties`
- **Status fields** (API-managed): `health`, `state`, `functional`, `temperature`, `conditions`, `errors`
- **Metadata** (API-managed): `uid`, `name`, `createdAt`, `modifiedAt`, `labels`, `annotations`

Status updates happen automatically based on:
- Hardware monitoring and health checks
- Business logic in the service
- External system integrations
- Condition controllers

### 6. Delete an FRU

```bash
# Delete FRU using the CLI
./fru-cli fru delete $FRU_UID
```

Expected output:
```
FRU fru-a1b2c3d4 deleted successfully
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

## Database Management

### View Database Contents

```bash
# Connect to SQLite database
sqlite3 fru.db

# List all tables
.tables

# View FRU resources
SELECT * FROM resources WHERE type = 'FRU';

# View labels and annotations
SELECT * FROM labels;
SELECT * FROM annotations;

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

### Issue: "sqlite: foreign_keys pragma is off: missing '_fk=1' in the connection string"

**Cause:** SQLite foreign keys are not enabled in the database connection
**Fix:** Use the correct database URL format with foreign keys enabled:
```bash
./fru-server serve --database-url "file:data/fru.db?_fk=1"
```

### Issue: "failed to open database: unable to open database file"

**Cause:** SQLite file path or permissions issue
**Fix:** Ensure the directory is writable:
```bash
mkdir -p data
./fru-server serve --database-url "file:data/fru.db?_fk=1"
```

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
    enabled: false         # Add custom auth middleware if needed
    provider: custom       # Implement your own auth provider

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

The basic example above works without authentication. To add custom authentication/authorization:

### Option 1: Add Custom Authorization Middleware

1. Create your authorization middleware in `internal/middleware/`
2. Implement your policy checking logic
3. Apply middleware to protected routes in `routes_generated.go`
4. Example: JWT-based authorization, RBAC, or ABAC

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
- [ ] Add custom authorization middleware
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
- ✅ **Kubernetes-Style Conditions** - Standardized condition tracking for complex state
- ✅ **Type Safety** - Compile-time validation throughout
- ✅ **REST API** - Full CRUD operations with OpenAPI spec
- ✅ **Client Library** - Generated CLI and programmatic client
- ✅ **Best Practices** - Generated code follows Go idioms

All generated from a simple resource definition! Add authentication and authorization as needed for your use case.
