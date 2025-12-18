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
- `apis.yaml` configuration for API groups and versions
- Flattened envelope structure with explicit `APIVersion`, `Kind`, `Metadata` fields

**Version Iteration**: Easy workflow for evolving your API:
```bash
# Start with a new major (alpha)
fabrica add resource Device --version v2alpha1

# Evolve to beta
fabrica add version v2beta1 --from v2alpha1

# Promote to stable (v2)
fabrica add version v2 --from v2beta1 --force
```

## Scenario: Device Management API

We're building a device management API that needs to support:
- **v2alpha1**: Early alpha for the v2 API
- **v2beta1**: Beta version with refined schema
- **v1**: Existing stable version (current storage/hub)

## Prerequisites

- Fabrica installed (see main README)
- Go 1.21 or later

## Project Structure

```
device-api/
├── .fabrica.yaml               # Feature flags (storage, events, etc.)
├── apis.yaml                   # API groups and versions
├── go.mod
├── cmd/
│   └── server/
│       └── main.go             # Server entry point
└── apis/                       # All versioned types
  └── infra.example.io/
    ├── v2alpha1/
    │   └── device_types.go # Alpha version types for v2
    ├── v2beta1/
    │   └── device_types.go # Beta version types for v2
    └── v1/                 # Hub (existing stable storage version)
      └── device_types.go # Stable version types
```

## Step-by-Step Guide

### 1. Initialize Versioned Project

```bash(default)
fabrica init device-api \
  --module github.com/user/device-api \
  --group infra.example.io

cd device-api
```
The generated `apis.yaml` includes:
```yaml
groups:
  - name: infra.example.io
    storageVfra.example.io
    storage_version: v1
    versions:
      - v1
    resources: []
```

**Optional**: Use `--versions v1,v2alpha1,v2beta1` to initialize with multiple versions from the start.

### 2. Add Resource to Storage Version

```bash
# Add Device resource (auto-selects storage hub v1)
fabrica add resource Device
```

Output:
```
No version specified, using storage hub version: v1
📦 Adding resource Device to infra.example.io/v1...
  ✓ Added Device to .fabrica.yaml

✅ Resource added sucapis.yaml

✅ Resource added successfully!

Next steps:
  1. Edit apis/infra.example.io/v1/device_types.go to customize your resource
  2. Add to other versions with 'fabrica add version <new-version>'
  3. Run 'fabrica generate' to create handlers
```

This creates `apis/infra.example.io/v1/device_types.go` and updates `apis.yaml

Edit `apis/infra.example.io/v1/device_types.go` to add your fields:

```go
type DeviceSpec struct {
    IPAddress   string            `json:"ipAddress" validate:"required,ip"`
    Location    string            `json:"location,omitempty"`
    DeviceType  string            `json:"deviceType" validate:"oneof=server switch router"`
    Tags        map[string]string `json:"tags,omitempty"`
    Description string            `json:"description,omitempty"`
}

type DeviceStatus struct {
    Phase       string              `json:"phase,omitempty"`
    Message     string              `json:"message,omitempty"`
    Ready       bool                `json:"ready"`
    LastChecked string              `json:"lastChecked,omitempty"`
    Conditions  []fabrica.Condition `json:"conditions,omitempty"`
}
```

### 4. (Optional) Add Pre-release Version for Next Major

To demonstrate version evolution, add `v2alpha1` (pre-release for v2).

**First, manually add v2alpha1** to `.fabrica.yaml`:

``Use the CLI to add the version**:

```bash
# Add v2alpha1 version
fabrica add version v2alpha1

# Add Device to the new version
fabrica add resource Device --version v2alpha1
```

This:
1. Updates `apis.yaml` to include v2alpha1 in the versions list
2. Creates `apis/infra.example.io/v2alpha1/device_types.go`
3. You can then customize it with experimental v2 features

Add v2beta1 to demonstrate progression from alpha → beta:

```yaml
# Update .fabrica.yaml
features:
  versioning:
    versions:
      - v1
      - v2alpha1
      - v2beta1  # Add this
```bash
# Add beta version based on alpha
fabrica add version v2beta1 --from v2alpha1

# This copies types from v2alpha1 to
```yaml
# Update .fabrica.yaml
features:
  versioning:
    storage_version: v2  # Change hub to v2
    versions:
      - v1               # Keep for backward compatibility
      - v2alpha1         # Can be removed once v2 is stable
      - v2beta1          # Can be removed once v2 is stable
      - v2               # Add stable v2
```

```bash
fabrica add resource Device --version v2 --force
```
bash
# Add stable v2 version (requires --force for stable versions)
fabrica add version v2 --from v2beta1 --force
```

Then manually update `apis.yaml` to change the storage hub:

```yaml
groups:
  - name: infra.example.io
    storageVersion: v2  # Change hub to v2
    versions:
      - v1              # Keep for backward compatibility
      - v2alpha1        # Can be removed once v2 is stable
      - v2beta1         # Can be removed once v2 is stable
      - v2              # Stable v2
```

After this change:s generates:
- Handlers in `cmd/server/*_handlers_generated.go`
- Storage layer in `internal/storage/storage_generated.go`
- Routes in `cmd/server/routes_generated.go`
- Client library in `pkg/client/client_generated.go`
- OpenAPI spec in `cmd/server/openapi_generated.go`
- Resource registration in `pkg/resources/register_generated.go`

Generated registration imports from the hub (storage) version:

```go
// Code generated by fabrica. DO NOT EDIT.
package resources

import (
    "fmt"
    "github.com/openchami/fabrica/pkg/codegen"
    v1 "github.com/user/device-api/apis/infra.example.io/v1"
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
1. Auto-selects storage_version hub (e.g., `v1`)
2. This ensures resources are added to the canonical storage version by default

```bash
# Auto-selects storage hub (v1 in this example)
fabrica add resource Device

# Explicitly specify a pre-release version if needed
fabrica add resource Device --version v2alpha1
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
