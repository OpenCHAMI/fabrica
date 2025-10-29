<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 1: Basic CRUD Operations

**Time to complete:** ~10 minutes
**Difficulty:** Beginner
**Prerequisites:** Go 1.23+, fabrica CLI installed

## What You'll Build

A device inventory API with full CRUD operations for managing network devices. This example demonstrates the complete workflow from initialization to a working API server using Fabrica's code generation.

## Step-by-Step Guide

### Step 1: Create Project Structure

```bash
# Basic initialization with defaults
fabrica init device-inventory
cd device-inventory
```

**Available init flags:**
- `--validation-mode`: Set validation mode (strict/warn/disabled, default: strict)
- `--events`: Enable CloudEvents support
- `--events-bus`: Event bus type (memory/nats/kafka, default: memory)
- `--version-strategy`: API versioning strategy (header/url/both, default: header)
- `--storage`: Storage backend (file/ent, default: file)
- `--db-driver`: Database driver for ent storage (sqlite/postgres/mysql)
- `--module`: Go module path (default: github.com/user/project-name)

**What `fabrica init` creates:**
```
device-inventory/
├── .fabrica.yaml           # Configuration file (NEW!)
├── cmd/server/main.go      # Server with commented storage/routes
├── pkg/resources/          # Empty (for your resources)
├── go.mod
└── docs/
```

The generated `.fabrica.yaml` includes:
- Project metadata (name, module, creation date)
- Feature configuration (validation mode, versioning strategy, events)
- Storage settings (file/ent, database driver)
- Generation options (package name, output directory)

**Default configuration:**
- ✅ Validation: strict mode (returns 400 on validation errors)
- ✅ Conditional requests: ETags with sha256 algorithm
- ✅ Versioning: header-based (`Accept: application/vnd.app.v1+json`)
- ❌ Events: disabled by default (can enable with `--events`)

The generated `main.go` includes:
- Chi router setup
- Storage initialization
- Route registration

### Step 2: Add a Resource

```bash
fabrica add resource Device
```

**What `fabrica add resource` creates:**

`pkg/resources/device/device.go`:
```go
package device

import (
    "context"
    "github.com/openchami/fabrica/pkg/resource"
)

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec" validate:"required"`
    Status DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Description string `json:"description,omitempty" validate:"max=200"`
    // Add your spec fields here
}

type DeviceStatus struct {
    Phase   string `json:"phase,omitempty"`
    Message string `json:"message,omitempty"`
    Ready   bool   `json:"ready"`
    // Add your status fields here
}

func (r *Device) Validate(ctx context.Context) error {
    // Add custom validation logic here
    return nil
}

func init() {
    resource.RegisterResourcePrefix("Device", "dev")
}
```

### Step 3: Customize Your Resource

Edit `pkg/resources/device/device.go` to add domain-specific fields.

```go
type DeviceSpec struct {
    Description string `json:"description,omitempty" validate:"max=200"`
    IPAddress   string `json:"ipAddress,omitempty" validate:"omitempty,ip"`
    Location    string `json:"location,omitempty"`
    Rack        string `json:"rack,omitempty"`
}
```

### Step 4: Generate Code

```bash
fabrica generate
```

### Step 4a: Update Dependencies

After generation, update your Go module dependencies:

```bash
go mod tidy
```

This resolves all the new imports that were added by the code generator.

**What `fabrica generate` creates:**

```
device-inventory/
├── cmd/server/
│   ├── main.go (unchanged - you'll edit this)
│   ├── device_handlers_generated.go    # CRUD handlers
│   ├── models_generated.go             # Request/response models
│   ├── routes_generated.go             # Route registration
│   └── openapi_generated.go            # OpenAPI spec
├── internal/
│   ├── middleware/                     # NEW! Core middleware
│   │   ├── validation_middleware_generated.go    # Request validation
│   │   ├── conditional_middleware_generated.go   # ETags/If-Match
│   │   └── versioning_middleware_generated.go    # API versioning
│   └── storage/
│       └── storage_generated.go        # File-based storage
└── pkg/resources/
    ├── device/device.go (your resource)
    └── register_generated.go            # Resource registry
```

The generator reads `.fabrica.yaml` and generates middleware based on your configuration:
- Validation middleware with your chosen mode (strict/warn/disabled)
- Conditional requests with ETag generation
- API versioning with your chosen strategy (header/url/both)
- Event bus setup if enabled


### Step 5: (Optional) Configure Features

Edit `.fabrica.yaml` to customize behavior:

```yaml
project:
  name: device-inventory
  module: github.com/user/device-inventory

features:
  validation:
    enabled: true
    mode: strict  # Options: strict, warn, disabled

  conditional:
    enabled: true
    etag_algorithm: sha256  # Options: sha256, md5

  versioning:
    enabled: true
    strategy: header  # Options: header, url, both

  events:
    enabled: false  # Set to true to enable CloudEvents
    bus_type: memory  # Options: memory, nats, kafka

  storage:
    type: file  # Options: file, ent
    db_driver: sqlite  # Options: sqlite, postgres, mysql
```

**Validation modes:**
- `strict`: Returns 400 Bad Request on validation errors (default)
- `warn`: Logs warnings but allows requests through
- `disabled`: No validation performed

**Version strategies:**
- `header`: Uses Accept header (e.g., `Accept: application/vnd.myapp.v1+json`)
- `url`: Uses URL prefix (e.g., `/v1/devices`)
- `both`: Supports both methods (header takes precedence)

### Step 6: Build Server and Client

```bash
# Build the server
go build -o server ./cmd/server

# Generate the client CLI
fabrica generate --client

# Build the client
go build -o client ./cmd/client
```

The server starts on port 8080 with:
- ✅ Full CRUD handlers
- ✅ File-based storage in `./data/`
- ✅ Request validation
- ✅ OpenAPI spec at `/openapi.json`

The client CLI provides:
- ✅ Type-safe commands for each resource
- ✅ JSON output formatting
- ✅ Helpful examples with `--help`

### Step 7: Run the Server

In one terminal:
```bash
./server
```

### Step 8: Test with the Generated Client

In another terminal:

```bash
# See what commands are available
./client --help

# Get help for device commands (shows spec field examples!)
./client device create --help

# Create a device
./client device create --spec '{
  "description": "Core network switch",
  "ipAddress": "192.168.1.10",
  "location": "DataCenter A",
  "rack": "R42"
}'

# List all devices
./client device list

# Get the UID from the list output, then get specific device
DEVID=$(./client device list | jq -r '.[0].metadata.uid')
./client device get $DEVID

# Update device
./client device update $DEVID --spec '{
  "description": "Updated description",
  "ipAddress": "192.168.1.20",
  "location": "DataCenter B"
}'

# Delete device
./client device delete $DEVID
```

**Alternative: Using curl**

If you prefer curl commands:

```bash
# Create a device
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "name": "switch-01",
    "description": "Core network switch",
    "ipAddress": "192.168.1.10",
    "location": "DataCenter A",
    "rack": "R42"
  }'

# List devices
curl http://localhost:8080/devices

# Get, update, and delete work the same way
```

## Understanding the Generated Code

### Middleware (`internal/middleware/`)

The generator creates middleware based on your `.fabrica.yaml` configuration:

**Validation Middleware** (`validation_middleware_generated.go`):
- Validates request payloads using struct tags
- Mode-aware behavior:
  - `strict`: Returns 400 Bad Request with validation details
  - `warn`: Logs warnings but continues processing
  - `disabled`: Skips validation entirely
- Integrates with `pkg/validation` package

**Conditional Middleware** (`conditional_middleware_generated.go`):
- Generates ETags for responses (sha256 or md5)
- Handles `If-Match` headers for conditional updates (returns 412 on mismatch)
- Handles `If-None-Match` headers for conditional GET (returns 304 if unchanged)
- Sets `Cache-Control` headers
- Prevents lost update problem with optimistic locking

**Versioning Middleware** (`versioning_middleware_generated.go`):
- Negotiates API version from request
- Supports header-based (`Accept: application/vnd.myapp.v1+json`)
- Supports URL-based (`/v1/devices`)
- Returns 406 Not Acceptable for unsupported versions
- Sets `X-API-Version` response header

**Event Bus** (`event_bus_generated.go`) - if events enabled:
- Initializes CloudEvents publisher
- Provides `PublishEvent()` and `PublishResourceEvent()` helpers
- Supports memory, NATS, or Kafka backends
- Publishes lifecycle events (created, updated, deleted)

### Client CLI (`cmd/client/main.go`)

The generated client provides a production-ready CLI tool:

```bash
# See available commands
./client --help

# Get command-specific help with field examples
./client device create --help
```

**What you get:**
- Commands for each resource (list, get, create, update, delete)
- Auto-generated examples showing **actual spec fields** from your resource
- Support for both stdin and `--spec` flag
- JSON output formatting
- Server URL configuration via flag or env var

**Example help output:**
```
Create a new Device.

Examples:
  # Create from stdin
  echo '{"description": "...", "ipAddress": "192.168.1.1"}' | inventory-cli device create

  # Create with --spec flag
  inventory-cli device create --spec '{"description": "...", "ipAddress": "192.168.1.1"}'

Spec fields:
  description (string)
  ipAddress (string)
  location (string)
  rack (string)
```

The help text automatically reflects your actual DeviceSpec fields!

### Handlers (`device_handlers_generated.go`)

Generated handlers include:
- **CreateDevice**: Creates resource, validates, generates UID, initializes metadata
- **GetDevice**: Retrieves by UID
- **ListDevices**: Returns all resources
- **UpdateDevice**: Updates spec fields, preserves metadata
- **DeleteDevice**: Removes from storage

### Storage (`storage_generated.go`)

File-based storage provides:
- Thread-safe operations with mutex
- JSON serialization
- Automatic directory creation
- Load/Save/Delete/List operations per resource type

### Models (`models_generated.go`)

Request/response models:
- **CreateDeviceRequest**: Embeds DeviceSpec inline, adds name/labels/annotations
- **UpdateDeviceRequest**: All fields optional for partial updates
- **DeviceResponse**: Type alias to device.Device

### Routes (`routes_generated.go`)

```go
func RegisterGeneratedRoutes(r chi.Router) {
    r.Route("/devices", func(r chi.Router) {
        r.Post("/", CreateDevice)
        r.Get("/", ListDevices)
        r.Get("/{uid}", GetDevice)
        r.Put("/{uid}", UpdateDevice)
        r.Delete("/{uid}", DeleteDevice)
    })
}
```

## Generated vs Manual Code

| Component | Generated? | Notes |
|-----------|-----------|-------|
| Project structure | ✅ `fabrica init` | Creates skeleton + `.fabrica.yaml` |
| Configuration file | ✅ `fabrica init` | `.fabrica.yaml` with defaults |
| Resource definition | ⚠️ Partial | `fabrica add resource` creates template, you customize |
| Registration file | ✅ `fabrica generate` | Auto-discovers resources |
| HTTP handlers | ✅ `fabrica generate` | Full CRUD operations |
| Validation middleware | ✅ `fabrica generate` | Mode-aware validation from config |
| Conditional middleware | ✅ `fabrica generate` | ETags, If-Match, cache control |
| Versioning middleware | ✅ `fabrica generate` | API version negotiation |
| Event bus | ⚠️ Conditional | Only if `events.enabled: true` in config |
| Request/response models | ✅ `fabrica generate` | Type-safe models |
| Storage backend | ✅ `fabrica generate` | File or Ent based on config |
| Route registration | ✅ `fabrica generate` | Chi router setup |
| OpenAPI spec | ✅ `fabrica generate` | Full API documentation |
| Go client library | ✅ `fabrica generate --client` | Type-safe HTTP client |
| CLI tool | ✅ `fabrica generate --client` | Cobra-based commands with examples |
| Server main.go | ⚠️ Manual | Uncomment generated imports/calls |

## Complete Workflow Summary

```bash
# 1. Initialize project (creates .fabrica.yaml with defaults)
fabrica init device-inventory
cd device-inventory

# 2. (Optional) Customize configuration
vim .fabrica.yaml  # Edit validation mode, versioning strategy, etc.

# 3. Add resource
fabrica add resource Device

# 4. Customize resource (edit pkg/resources/device/device.go)
#    - Remove Name from DeviceSpec
#    - Add your domain fields

# 5. Generate everything (reads .fabrica.yaml for config)
fabrica generate

# 6. Update dependencies
go mod tidy

# 7. Uncomment in cmd/server/main.go:
#    - import "github.com/user/device-inventory/internal/storage"
#    - storage.InitFileBackend("./data")
#    - RegisterGeneratedRoutes(r)

# 8. Build server and client
go build -o server ./cmd/server
fabrica generate --client
go build -o client ./cmd/client

# 9. Run and test
./server  # In one terminal
./client device list  # In another terminal

# 10. (Optional) Modify .fabrica.yaml and regenerate
vim .fabrica.yaml  # Change validation mode, enable events, etc.
fabrica generate   # Regenerate with new config
go build ./cmd/server
```

## Key Features

✅ **Zero boilerplate** - Generate complete CRUD in seconds
✅ **Type-safe** - Compile-time validation of all operations
✅ **Kubernetes-style** - Resources with APIVersion, Kind, Metadata, Spec, Status
✅ **Configuration-driven** - `.fabrica.yaml` controls all feature generation
✅ **Validation middleware** - Configurable strict/warn/disabled modes
✅ **Conditional requests** - ETags, If-Match, If-None-Match, 304 Not Modified
✅ **API versioning** - Header, URL, or both strategies supported
✅ **Event support** - Optional CloudEvents integration (memory/NATS/Kafka)
✅ **Storage abstraction** - File-based or Ent (database) storage
✅ **OpenAPI** - Auto-generated API documentation
✅ **Client SDK** - Generated Go client library and CLI tool with helpful examples

## Advanced Configuration

### Changing Validation Mode

To switch from strict to warn mode (useful for gradual validation rollout):

```yaml
# .fabrica.yaml
features:
  validation:
    enabled: true
    mode: warn  # Changed from strict
```

Then regenerate:
```bash
fabrica generate
go build ./cmd/server
```

The server will now log validation errors but continue processing requests.

### Enabling Events

To add event publishing to your API:

```yaml
# .fabrica.yaml
features:
  events:
    enabled: true
    bus_type: memory  # Start with memory, upgrade to nats/kafka later
```

Regenerate and the event bus will be initialized. You can publish events from handlers:

```go
import "github.com/user/device-inventory/internal/middleware"

// In your handler
middleware.PublishResourceEvent(ctx, "created", "Device", device.UID, device)
```

### Switching to URL-based Versioning

To use `/v1/devices` style URLs instead of Accept headers:

```yaml
# .fabrica.yaml
features:
  versioning:
    enabled: true
    strategy: url  # Changed from header
```

The generated middleware will now parse version from URL path.

### Upgrading to Database Storage

To switch from file storage to Ent with PostgreSQL:

```yaml
# .fabrica.yaml
features:
  storage:
    type: ent
    db_driver: postgres
```

Then regenerate and update your connection string in `main.go`.

## Common Issues

### Issue: `validation failed: name is required`

**Cause:** DeviceSpec still has `Name` field
**Fix:** Remove Name from DeviceSpec - the name belongs in metadata!

```go
// ❌ Wrong
type DeviceSpec struct {
    Name        string `json:"name" validate:"required"`
    Description string `json:"description"`
}

// ✅ Correct
type DeviceSpec struct {
    Description string `json:"description"`
    // Name is in metadata, not spec!
}
```

### Issue: `context imported but not used`

**Cause:** Old template bug (fixed in current version)
**Fix:** Run `fabrica generate` with latest version

### Issue: Generated code has `// Spec: TODO`

**Cause:** Old template bug (fixed in current version)
**Fix:** Rebuild fabrica CLI with latest templates

### Issue: Middleware not generated

**Cause:** `.fabrica.yaml` missing or invalid
**Fix:** Ensure `.fabrica.yaml` exists with proper structure. Run `fabrica init` again if needed:

```bash
# Reinitialize config only (won't overwrite existing code)
fabrica init . --module github.com/user/device-inventory
```

### Issue: Changes to `.fabrica.yaml` not reflected

**Cause:** Need to regenerate code after config changes
**Fix:** Always run `fabrica generate` after editing `.fabrica.yaml`:

```bash
vim .fabrica.yaml  # Edit configuration
fabrica generate   # Regenerate with new config
go build ./cmd/server
```

## Next Steps

- Add more resources with `fabrica add resource`
- Try the authentication example: [Example 2 - Storage & Auth](../02-storage-auth/)
- Implement reconciliation loops: [Example 3 - Workflows](../03-workflows/)
- Customize validation in your resource's `Validate()` method
- Add custom handlers beyond generated CRUD

## Development Tips

### Working with Local Fabrica Source

If developing Fabrica itself, add a replace directive to your test project's `go.mod`:

```go
replace github.com/openchami/fabrica => /path/to/local/fabrica
```

This ensures `fabrica generate` uses your local templates instead of the published version.

### Regenerating After Resource Changes

After modifying your resource definition:

```bash
fabrica generate  # Regenerates all code
go build ./cmd/server
```

The generator is idempotent - safe to run multiple times.

## Summary

Fabrica's code generation creates production-ready CRUD APIs from simple resource definitions. The workflow is fast, type-safe, and follows Kubernetes conventions. Customize resources to match your domain, generate handlers/storage/routes, and you have a working API!
