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

**Note:** This example focuses on the core Fabrica features. Casbin and TokenSmith integration mentioned in testing sections are advanced features that require additional manual setup beyond the generated code

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

**Note:** Ent client code generation now runs automatically when Ent storage is detected. The `fabrica ent generate` command is deprecated but still available for backward compatibility.

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

### Step 5: Uncomment Server Setup in main.go

The generated `cmd/server/main.go` has storage and routing code commented out. You need to uncomment these sections to activate them.

#### Step 6a: Uncomment Storage Imports

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

#### Step 6b: Uncomment Storage Initialization

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

#### Step 6c: Uncomment Route Registration

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

### Step 7: Verify Dependencies

The required dependencies should now be installed:
- `entgo.io/ent` - ORM framework
- `github.com/mattn/go-sqlite3` - SQLite driver
- `github.com/casbin/casbin/v2` - Authorization
- `github.com/OpenCHAMI/tokensmith/middleware` - JWT authentication

### Step 8: Build and Run

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

### Step 9: Build Client CLI

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

**Note:** The CLI supports getting resources by UID only. Name-based lookups would require additional implementation in the generated code.

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

### 6. Working with Conditions

#### Setting Multiple Conditions

Create an FRU with comprehensive condition tracking:

```bash
# Create memory FRU with conditions using --spec
./fru-cli fru create --spec '{
  "name": "memory-001",
  "fruType": "Memory",
  "serialNumber": "MEM12345",
  "partNumber": "DDR4-3200",
  "manufacturer": "Samsung",
  "model": "32GB DDR4",
  "location": {
    "rack": "R42",
    "chassis": "C1",
    "slot": "U10",
    "socket": "DIMM_A1"
  },
  "status": {
    "health": "OK",
    "state": "Present",
    "functional": "Enabled",
    "temperature": 45.0,
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "reason": "MemoryOnline",
        "message": "Memory module is ready for use",
        "lastTransitionTime": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      },
      {
        "type": "Healthy",
        "status": "True",
        "reason": "TemperatureNormal",
        "message": "Operating temperature within normal range (45°C)",
        "lastTransitionTime": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      },
      {
        "type": "Reachable",
        "status": "True",
        "reason": "BMCConnected",
        "message": "Memory accessible via BMC DIMM sensors",
        "lastTransitionTime": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      }
    ]
  }
}'
```

Alternatively, create from a comprehensive JSON file:

```bash
cat > memory-with-conditions.json <<EOF
{
  "name": "memory-001",
  "fruType": "Memory",
  "serialNumber": "MEM12345",
  "partNumber": "DDR4-3200",
  "manufacturer": "Samsung",
  "model": "32GB DDR4",
  "location": {
    "rack": "R42",
    "chassis": "C1",
    "slot": "U10",
    "socket": "DIMM_A1"
  },
  "status": {
    "health": "OK",
    "state": "Present",
    "functional": "Enabled",
    "temperature": 45.0,
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "reason": "MemoryOnline",
        "message": "Memory module is ready for use",
        "lastTransitionTime": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      },
      {
        "type": "Healthy",
        "status": "True",
        "reason": "TemperatureNormal",
        "message": "Operating temperature within normal range (45°C)",
        "lastTransitionTime": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      },
      {
        "type": "Reachable",
        "status": "True",
        "reason": "BMCConnected",
        "message": "Memory accessible via BMC DIMM sensors",
        "lastTransitionTime": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      }
    ]
  }
}
EOF

cat memory-with-conditions.json | ./fru-cli fru create
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

#### Working with Conditions

**Note:** Conditions are status fields managed by the API. The examples below show what the API might set automatically:

```bash
# View current conditions
./fru-cli fru get $FRU_UID --output json | jq '.status.conditions'
```

Expected condition structure (API-managed):
```json
{
  "status": {
    "conditions": [
      {
        "type": "Ready",
        "status": "True",
        "reason": "MemoryOnline",
        "message": "Memory module is ready for use",
        "lastTransitionTime": "2025-10-10T12:05:00Z"
      }
    ]
  }
}
```

#### Updating Conditions for Failures

Update conditions to reflect a failure state:

```bash
# Get the FRU UID from previous command
FRU_UID="fru-a1b2c3d4"

# IMPORTANT: Conditions and status are API-managed, not user-modifiable
# The following shows what the API might set automatically during failure detection

# Instead of updating status directly, you might update spec fields that trigger status changes:
./fru-cli fru patch $FRU_UID --spec '{
  "properties": {
    "maintenanceMode": "true",
    "lastMaintenanceReason": "ECC errors detected"
  }
}'

# The API would then automatically update status.conditions based on:
# - Hardware monitoring detecting ECC errors
# - Temperature sensors reporting critical levels
# - Health check failures
#
# Resulting in conditions like:
# {
#   "type": "Ready",
#   "status": "False",
#   "reason": "MemoryErrors",
#   "message": "Memory module has ECC errors and is not reliable"
# }
```

#### Recovery and Progress Tracking

Track recovery operations with spec updates that trigger API status management:

```bash
# Update spec to indicate maintenance operations
./fru-cli fru patch $FRU_UID --spec '{
  "properties": {
    "maintenanceMode": "true",
    "maintenanceType": "ECC_SCRUBBING",
    "maintenanceStarted": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "expectedDuration": "5m"
  }
}'

# The API monitoring system would then automatically update status to reflect:
# - Health changing to "Warning" (maintenance in progress)
# - Conditions showing maintenance state:
#   {
#     "type": "Ready",
#     "status": "False",
#     "reason": "MaintenanceMode",
#     "message": "Memory module in maintenance mode for ECC scrubbing"
#   },
#   {
#     "type": "Progressing",
#     "status": "True",
#     "reason": "ECCScrubInProgress",
#     "message": "ECC scrub operation 65% complete (estimated 2 minutes remaining)"
#   }

# When maintenance completes, update spec to clear maintenance flags:
./fru-cli fru patch $FRU_UID --spec '{
  "properties": {
    "maintenanceMode": "false",
    "lastMaintenanceCompleted": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "maintenanceResult": "SUCCESS"
  }
}'
```

#### Condition Best Practices

**Common Condition Types for FRUs:**
- `Ready`: FRU is ready for normal operation
- `Healthy`: FRU is functioning within normal parameters
- `Reachable`: FRU can be accessed for management
- `Progressing`: FRU is making progress toward desired state
- `Available`: FRU is available for allocation/use
- `Degraded`: FRU is functional but with reduced capability

**Status Guidelines:**
- Use `"True"` when the condition is satisfied
- Use `"False"` when the condition is explicitly not satisfied
- Use `"Unknown"` when the condition status cannot be determined
- Always include meaningful `reason` and `message` fields
- Update `lastTransitionTime` only when status changes

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
    Conditions  []resource.Condition `json:"conditions,omitempty"` // Kubernetes-style conditions
}
```

### 3. Kubernetes-Style Conditions

The FRU resource includes support for **Conditions**, following the Kubernetes pattern for tracking detailed resource status. Conditions provide a standardized way to represent different aspects of a resource's state.

#### Understanding Conditions

Each condition has:
- **Type**: The aspect being tracked (e.g., "Ready", "Healthy", "Reachable")
- **Status**: Current state ("True", "False", "Unknown")
- **Reason**: Machine-readable reason code
- **Message**: Human-readable explanation
- **LastTransitionTime**: When the condition last changed status

#### Common FRU Condition Types

```go
// Example conditions for FRU resources
conditions := []resource.Condition{
    {
        Type:    "Ready",
        Status:  "True",
        Reason:  "FRUOnline",
        Message: "FRU is ready for use",
    },
    {
        Type:    "Healthy",
        Status:  "False",
        Reason:  "TemperatureHigh",
        Message: "CPU temperature exceeds 80°C threshold",
    },
    {
        Type:    "Reachable",
        Status:  "True",
        Reason:  "BMCResponding",
        Message: "FRU accessible via BMC management interface",
    },
    {
        Type:    "Progressing",
        Status:  "True",
        Reason:  "FirmwareUpdate",
        Message: "Firmware update in progress (45% complete)",
    },
}
```

#### Working with Conditions

**Creating Conditions:**
```go
import "github.com/alexlovelltroy/fabrica/pkg/resource"

// Create a new condition
condition := resource.NewCondition("Ready", "True", "FRUOnline", "FRU is operational")

// Add to FRU status
var conditions []resource.Condition
resource.SetCondition(&conditions, "Ready", "True", "FRUOnline", "FRU is operational")
```

**Checking Conditions:**
```go
// Check if FRU is ready
if resource.IsConditionTrue(fru.Status.Conditions, "Ready") {
    // FRU is ready for use
}

// Get condition status
status := resource.GetConditionStatus(fru.Status.Conditions, "Healthy")
switch status {
case "True":
    // FRU is healthy
case "False":
    // FRU has health issues
default: // "Unknown"
    // Health status unknown
}
```

**Updating Conditions:**
```go
// Update existing condition or create new one
resource.SetCondition(&fru.Status.Conditions, "Healthy", "False", "OverTemp", "Temperature critical: 85°C")

// Remove a condition
resource.RemoveCondition(&fru.Status.Conditions, "Progressing")
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

### Conditions Quick Reference

#### Creating Conditions
```go
import "github.com/alexlovelltroy/fabrica/pkg/resource"

// Create new condition
condition := resource.NewCondition("Ready", "True", "FRUOnline", "FRU is operational")

// Set condition on resource
resource.SetCondition(&fru.Status.Conditions, "Ready", "True", "FRUOnline", "FRU is operational")
```

#### Checking Conditions
```go
// Check if condition is true
if resource.IsConditionTrue(fru.Status.Conditions, "Ready") {
    // Handle ready state
}

// Get condition status
status := resource.GetConditionStatus(fru.Status.Conditions, "Healthy")
// Returns: "True", "False", or "Unknown"

// Find specific condition
condition := resource.FindCondition(fru.Status.Conditions, "Ready")
if condition != nil && condition.IsTrue() {
    // Handle condition
}
```

#### Common Patterns
```go
// Set multiple conditions
resource.SetCondition(&conditions, "Ready", "True", "Online", "FRU ready")
resource.SetCondition(&conditions, "Healthy", "True", "Normal", "All checks passed")
resource.SetCondition(&conditions, "Reachable", "True", "Connected", "BMC responding")

// Update condition status
resource.SetCondition(&conditions, "Healthy", "False", "OverTemp", "Temperature critical")

// Remove condition
resource.RemoveCondition(&conditions, "Progressing")
```

#### Status Values
- **"True"**: Condition is satisfied
- **"False"**: Condition is explicitly not satisfied
- **"Unknown"**: Condition status cannot be determined

#### Common Condition Types
- **Ready**: Resource ready for normal operation
- **Healthy**: Resource functioning within normal parameters
- **Reachable**: Resource accessible for management
- **Progressing**: Resource making progress toward desired state
- **Available**: Resource available for allocation
- **Degraded**: Resource functional but with reduced capability

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
- ✅ **Kubernetes-Style Conditions** - Standardized condition tracking for complex state
- ✅ **Type Safety** - Compile-time validation throughout
- ✅ **REST API** - Full CRUD operations with OpenAPI spec
- ✅ **Client Library** - Generated CLI and programmatic client
- ✅ **Best Practices** - Generated code follows Go idioms

All generated from a simple resource definition! Add authentication and authorization as needed for your use case.
