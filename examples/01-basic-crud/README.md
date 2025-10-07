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
fabrica init device-inventory
cd device-inventory
```

**What `fabrica init` creates:**
```
device-inventory/
├── cmd/server/main.go     # Server with commented storage/routes
├── pkg/resources/         # Empty (for your resources)
├── go.mod
└── docs/
```

The generated `main.go` includes:
- Chi router setup
- Commented storage initialization (uncomment after generate)
- Commented route registration (uncomment after generate)

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
    "github.com/alexlovelltroy/fabrica/pkg/resource"
)

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec" validate:"required"`
    Status DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Name        string `json:"name" validate:"required,k8sname"`
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

**Important:** Remove the `Name` field from DeviceSpec - the name belongs in metadata, not the spec!

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

**What `fabrica generate` creates:**

```
device-inventory/
├── cmd/server/
│   ├── main.go (unchanged - you'll edit this)
│   ├── device_handlers_generated.go    # CRUD handlers
│   ├── models_generated.go             # Request/response models
│   ├── routes_generated.go             # Route registration
│   └── openapi_generated.go            # OpenAPI spec
├── internal/storage/
│   └── storage_generated.go            # File-based storage
└── pkg/resources/
    ├── device/device.go (your resource)
    └── register_generated.go            # Resource registry
```

### Step 5: Uncomment Storage & Routes in main.go

Edit `cmd/server/main.go` and uncomment the generated lines:

```go
package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/user/device-inventory/internal/storage"  // Uncomment this
)

func main() {
    // Uncomment storage initialization
    if err := storage.InitFileBackend("./data"); err != nil {
        log.Fatalf("Failed to initialize storage: %v", err)
    }

    r := chi.NewRouter()

    // Uncomment route registration
    RegisterGeneratedRoutes(r)

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}
```

### Step 6: Build and Run

```bash
go build -o server ./cmd/server
./server
```

The server starts on port 8080 with:
- ✅ Full CRUD handlers
- ✅ File-based storage in `./data/`
- ✅ Request validation
- ✅ OpenAPI spec at `/openapi.json`

### Step 7: Test the API

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

# List all devices
curl http://localhost:8080/devices

# Get specific device (use UID from create response)
curl http://localhost:8080/devices/dev-1ea29407

# Update device
curl -X PUT http://localhost:8080/devices/dev-1ea29407 \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated description",
    "ipAddress": "192.168.1.20",
    "location": "DataCenter B"
  }'

# Delete device
curl -X DELETE http://localhost:8080/devices/dev-1ea29407
```

## Understanding the Generated Code

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
| Project structure | ✅ `fabrica init` | Creates skeleton |
| Resource definition | ⚠️ Partial | `fabrica add resource` creates template, you customize |
| Registration file | ✅ `fabrica generate` | Auto-discovers resources |
| HTTP handlers | ✅ `fabrica generate` | Full CRUD operations |
| Request/response models | ✅ `fabrica generate` | Type-safe models |
| Storage backend | ✅ `fabrica generate` | File-based implementation |
| Route registration | ✅ `fabrica generate` | Chi router setup |
| OpenAPI spec | ✅ `fabrica generate` | Full API documentation |
| Server main.go | ⚠️ Manual | Uncomment generated imports/calls |

## Complete Workflow Summary

```bash
# 1. Initialize project
fabrica init device-inventory
cd device-inventory

# 2. Add resource
fabrica add resource Device

# 3. Customize resource (edit pkg/resources/device/device.go)
#    - Remove Name from DeviceSpec
#    - Add your domain fields

# 4. Generate everything
fabrica generate

# 5. Uncomment in cmd/server/main.go:
#    - import "github.com/user/device-inventory/internal/storage"
#    - storage.InitFileBackend("./data")
#    - RegisterGeneratedRoutes(r)

# 6. Build and run
go build -o server ./cmd/server
./server
```

## Key Features

✅ **Zero boilerplate** - Generate complete CRUD in seconds
✅ **Type-safe** - Compile-time validation of all operations
✅ **Kubernetes-style** - Resources with APIVersion, Kind, Metadata, Spec, Status
✅ **Validation** - Struct tags + custom validation hooks
✅ **Storage abstraction** - File-based by default, easily extended
✅ **OpenAPI** - Auto-generated API documentation

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
replace github.com/alexlovelltroy/fabrica => /path/to/local/fabrica
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
