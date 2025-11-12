<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 8: API Versioning

This example demonstrates Fabrica's API versioning system with a clean, unified architecture. You'll learn how to:

- Create versioned APIs from the start
- Add resources to specific API versions
- Iterate on API versions by copying and evolving them
- Manage multiple versions in a single configuration file

## What This Example Shows

**APIs-First Architecture**: Fabrica uses a single source of truth for versioned APIs:
- All types live in `apis/<group>/<version>/`
- No redundancy between `pkg/resources/` and `apis/`
- Single `.fabrica.yaml` configuration (no separate `apis.yaml`)
- Flattened envelope structure with explicit `APIVersion`, `Kind`, `Metadata` fields

**Version Iteration**: Easy workflow for evolving your API:
```bash
# Start with alpha version
fabrica add resource Device --version v1alpha1

# Evolve to beta
fabrica add version v1beta1 --from v1alpha1

# Promote to stable
fabrica add version v1 --from v1beta1 --force
```

## Scenario: Device Management API

We're building a device management API that needs to support:
- **v1alpha1**: Early alpha version with basic fields
- **v1beta1**: Beta version with refined schema
- **v1**: Stable version (storage version/hub)

## Prerequisites

- Fabrica installed (see main README)
- Go 1.21 or later

## Project Structure

```
device-api/
├── .fabrica.yaml               # Unified configuration
├── go.mod
├── cmd/
│   └── server/
│       └── main.go             # Server entry point
└── apis/                       # All versioned types
    └── infra.example.io/
        ├── v1alpha1/
        │   └── device_types.go # Alpha version types
        ├── v1beta1/
        │   └── device_types.go # Beta version types
        └── v1/                 # Hub (storage version)
            └── device_types.go # Stable version types
```

## Step-by-Step Guide

### 1. Initialize Versioned Project

```bash
# Create project with versioning enabled from the start
fabrica init device-api \
  --group infra.example.io \
  --storage-version v1 \
  --versions v1alpha1,v1beta1,v1

cd device-api
```

This creates:
- `.fabrica.yaml` with versioning configuration
- `apis/infra.example.io/v1alpha1/`, `v1beta1/`, `v1/` directories
- **No** `pkg/resources/` directory (versioned mode)

### 2. Add Resource to Alpha Version

```bash
# Add Device resource (auto-selects v1alpha1)
fabrica add resource Device
```

Output:
```
No version specified, using first alpha version: v1alpha1
📦 Adding resource Device to infra.example.io/v1alpha1...
  ✓ Added Device to .fabrica.yaml

✅ Resource added successfully!
```

This creates `apis/infra.example.io/v1alpha1/device_types.go`:

```go
package v1alpha1

import (
    "context"
    "github.com/openchami/fabrica/pkg/fabrica"
)

// Device represents a device resource
type Device struct {
    APIVersion string           `json:"apiVersion"`
    Kind       string           `json:"kind"`
    Metadata   fabrica.Metadata `json:"metadata"`
    Spec       DeviceSpec       `json:"spec" validate:"required"`
    Status     DeviceStatus     `json:"status,omitempty"`
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

func (r *Device) GetKind() string {
    return "Device"
}

func (r *Device) GetName() string {
    return r.Metadata.Name
}

func (r *Device) GetUID() string {
    return r.Metadata.UID
}

func (r *Device) Validate(ctx context.Context) error {
    return nil
}
```

### 3. Customize the Alpha Version

Edit `apis/infra.example.io/v1alpha1/device_types.go` to add your fields:

```go
type DeviceSpec struct {
    Name       string `json:"name" validate:"required"`
    IPAddress  string `json:"ipAddress" validate:"required,ip"`
    Location   string `json:"location,omitempty"`
    DeviceType string `json:"deviceType" validate:"oneof=server switch router"`
}

type DeviceStatus struct {
    Health      string `json:"health,omitempty"`
    LastChecked string `json:"lastChecked,omitempty"`
    Ready       bool   `json:"ready"`
}
```

### 4. Copy to Beta Version

```bash
# Copy v1alpha1 types to v1beta1
fabrica add version v1beta1 --from v1alpha1
```

Output:
```
📦 Adding version infra.example.io/v1beta1 (copying from v1alpha1)...
  ✓ Copied device_types.go
  ✓ Added v1beta1 to .fabrica.yaml

✅ Version added successfully!
```

This creates `apis/infra.example.io/v1beta1/device_types.go` with the package updated to `package v1beta1`.

### 5. Evolve the Beta Version

Edit `apis/infra.example.io/v1beta1/device_types.go` to refine the schema:

```go
type DeviceSpec struct {
    Name        string            `json:"name" validate:"required"`
    IPAddress   string            `json:"ipAddress" validate:"required,ip"`
    Location    string            `json:"location,omitempty"`
    DeviceType  string            `json:"deviceType" validate:"oneof=server switch router"`
    Tags        map[string]string `json:"tags,omitempty"` // NEW: Added tags
    Description string            `json:"description,omitempty"`
}

type DeviceStatus struct {
    Health      string      `json:"health,omitempty"`
    LastChecked string      `json:"lastChecked,omitempty"`
    Ready       bool        `json:"ready"`
    Conditions  []Condition `json:"conditions,omitempty"` // NEW: Added conditions
}

type Condition struct {
    Type    string `json:"type"`
    Status  string `json:"status"`
    Reason  string `json:"reason,omitempty"`
    Message string `json:"message,omitempty"`
}
```

### 6. Promote to Stable Version

```bash
# Add Device to v1 (hub/storage version)
fabrica add resource Device --version v1 --force
```

Note: Adding to non-alpha/beta versions requires `--force` flag.

Then edit `apis/infra.example.io/v1/device_types.go` to match your stable schema (copy from v1beta1).

### 7. Generate Code

```bash
fabrica generate
go mod tidy
```

This generates:
- Handlers in `pkg/handlers/device/`
- Storage interface in `pkg/storage/`
- Client in `pkg/client/`
- OpenAPI spec
- Registration code in `pkg/resources/register_generated.go`

Generated registration imports from the hub version:

```go
// Code generated by fabrica. DO NOT EDIT.
package resources

import (
    "fmt"
    "github.com/openchami/fabrica/pkg/codegen"
    v1 "github.com/example/device-api/apis/infra.example.io/v1"
)

func RegisterAllResources(gen *codegen.Generator) error {
    if err := gen.RegisterResource(&v1.Device{}); err != nil {
        return fmt.Errorf("failed to register Device: %w", err)
    }
    return nil
}
```

### 8. Run the Server

```bash
go run ./cmd/server
```

The server starts on `http://localhost:8080`.

### 9. Test the API

#### Create a Device

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1",
    "kind": "Device",
    "metadata": {"name": "device-1"},
    "spec": {
      "name": "device-1",
      "ipAddress": "192.168.1.100",
      "location": "DataCenter A",
      "deviceType": "server",
      "tags": {"env": "prod"}
    }
  }'
```

#### List All Devices

```bash
curl http://localhost:8080/devices
```

#### Get a Device

```bash
curl http://localhost:8080/devices/device-1
```

#### Update a Device

```bash
curl -X PUT http://localhost:8080/devices/device-1 \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1",
    "kind": "Device",
    "metadata": {"name": "device-1"},
    "spec": {
      "name": "device-1",
      "ipAddress": "192.168.1.101",
      "location": "DataCenter B",
      "deviceType": "switch",
      "tags": {"env": "staging"}
    }
  }'
```

#### Delete a Device

```bash
curl -X DELETE http://localhost:8080/devices/device-1
```

## Configuration Reference

### .fabrica.yaml

```yaml
project:
  name: device-api
  module: github.com/example/device-api
  description: Device management API
  created: "2025-11-12T12:00:00Z"

features:
  validation:
    enabled: true
    mode: strict

  versioning:
    enabled: true
    group: infra.example.io       # API group
    storage_version: v1            # Hub version (for storage)
    versions:                      # All versions
      - v1alpha1
      - v1beta1
      - v1
    resources:                     # Resource kinds
      - Device

  storage:
    enabled: true
    type: file

generation:
  handlers: true
  storage: true
  client: true
  openapi: true
  middleware: true
```

## Key Concepts

### Flattened Envelope Structure

Unlike the legacy mode where `resource.Resource` is embedded, versioned types use explicit fields with a shared `fabrica.Metadata` type:

```go
// Versioned type (explicit fields)
type Device struct {
    APIVersion string           `json:"apiVersion"` // "infra.example.io/v1"
    Kind       string           `json:"kind"`       // "Device"
    Metadata   fabrica.Metadata `json:"metadata"`   // Imported from pkg/fabrica
    Spec       DeviceSpec       `json:"spec"`
    Status     DeviceStatus     `json:"status,omitempty"`
}

// Legacy type (embedded)
type Device struct {
    resource.Resource                            // Embedded (includes all fields)
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status,omitempty"`
}
```

**Note**: The `fabrica.Metadata` type is shared across all resources and versioned APIs (aliased from `pkg/resource/metadata.go`). This provides a consistent metadata structure while avoiding duplication.

### Version Auto-Selection

When adding resources without `--version`:
1. Auto-selects first alpha version (e.g., `v1alpha1`)
2. If no alpha version exists, requires explicit `--version` with `--force`

```bash
# Auto-selects v1alpha1
fabrica add resource Device

# Requires --force for non-alpha
fabrica add resource Device --version v1 --force
```

### Storage Version (Hub)

The `storage_version` field defines which version is used for persistence:
- All data is stored in this format
- Should be a stable version (e.g., `v1`, not `v1alpha1`)
- Must be in the `versions` list

### Version Iteration Workflow

1. **Alpha**: Start with `v1alpha1`, iterate rapidly
2. **Beta**: Copy to `v1beta1` when semi-stable, refine schema
3. **Stable**: Copy to `v1` when ready for production, mark as `storage_version`
4. **Deprecation**: Remove old versions from `versions` list when no longer supported

## Comparison: Versioned vs Legacy Mode

### Versioned Mode (This Example)

```bash
fabrica init device-api --group infra.example.io --versions v1alpha1,v1
```

**Structure:**
```
device-api/
├── .fabrica.yaml               # Single config
└── apis/infra.example.io/
    ├── v1alpha1/
    │   └── device_types.go     # User-defined
    └── v1/
        └── device_types.go     # User-defined
```

**Benefits:**
- Single source of truth for types
- No redundancy
- Clear version ownership
- Easy to iterate on versions

### Legacy Mode

```bash
fabrica init device-api
```

**Structure:**
```
device-api/
├── .fabrica.yaml
└── pkg/resources/device/
    └── device.go               # User-defined (embeds resource.Resource)
```

**Use Case:**
- Simple projects without versioning needs
- Single API version
- Quick prototyping

## Troubleshooting

### Error: "version X not found in .fabrica.yaml"

**Cause**: Specified version doesn't exist in config.

**Solution**: Add version to `.fabrica.yaml` or use existing version:
```yaml
features:
  versioning:
    versions:
      - v1alpha1
      - v1beta1
      - v1          # Add your version here
```

### Error: "adding resource to non-alpha version requires --force"

**Cause**: Safety check to prevent accidentally adding to stable versions.

**Solution**: Use `--force` flag:
```bash
fabrica add resource Device --version v1 --force
```

### Error: "No resources found"

**Cause**: Hub version directory is empty.

**Solution**: Add resource to hub (storage) version:
```bash
fabrica add resource Device --version v1 --force
```

### Generator Shows "Legacy mode"

**Cause**: `.fabrica.yaml` has `versioning.enabled: false` or no versions defined.

**Solution**: Enable versioning:
```yaml
features:
  versioning:
    enabled: true
    group: infra.example.io
    storage_version: v1
    versions: [v1alpha1, v1]
    resources: [Device]
```

## Next Steps

- **Add More Resources**: `fabrica add resource Sensor --version v1alpha1`
- **Implement Conversions**: Add custom `ConvertTo()` and `ConvertFrom()` methods for non-trivial schema changes
- **Version Negotiation**: Add middleware to support multiple versions at runtime
- **Deprecation**: Remove old versions from `versions` list when ready
- **Documentation**: Add OpenAPI annotations to generate better API docs

## Learn More

- [Getting Started](../../docs/guides/getting-started.md)
- [Configuration Reference](../../docs/configuration.md)
- [Resource Management](../../docs/resources.md)
