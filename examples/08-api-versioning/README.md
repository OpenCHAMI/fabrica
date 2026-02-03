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
- No redundancy between `apis/` and generated code
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
    storageVersion: v1
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
  ✓ Added Device to apis.yaml

✅ Resource added successfully!

Next steps:
  1. Edit apis/infra.example.io/v1/device_types.go to customize your resource
  2. Add to other versions with 'fabrica add version <new-version>'
  3. Run 'fabrica generate' to create handlers
```

This creates `apis/infra.example.io/v1/device_types.go` and updates `apis.yaml`.

### 3. Customize the Resource Spec

Edit `apis/infra.example.io/v1/device_types.go` to add your fields:

```go
type DeviceSpec struct {
    IPAddress   string            `json:"ipAddress" validate:"required,ip"`
    Location    string            `json:"location,omitempty"`
    DeviceType  string            `json:"deviceType" validate:"required,oneof=server switch router"`
    Tags        map[string]string `json:"tags,omitempty"`
    Description string            `json:"description,omitempty"`
}

// Note: In validation tags, 'ip' is the validator function (validates IP addresses),
// NOT the field name. Common validators: ip, email, uuid, required, oneof.
// The JSON field name is 'ipAddress' (from the json tag).

type DeviceStatus struct {
    Phase       string              `json:"phase,omitempty"`
    Message     string              `json:"message,omitempty"`
    Ready       bool                `json:"ready"`
    LastChecked string              `json:"lastChecked,omitempty"`
    Conditions  []fabrica.Condition `json:"conditions,omitempty"`
}

// Note: Import fabrica.Condition from github.com/openchami/fabrica/pkg/fabrica
```

### 4. Generate the Server Code

Run `fabrica generate` to create the server implementation:

```bash
fabrica generate
```

This generates:
- Handlers in `cmd/server/*_handlers_generated.go`
- Routes in `cmd/server/routes_generated.go`
- Storage layer in `internal/storage/`
- Client library in `pkg/client/`
- OpenAPI specification in `cmd/server/openapi_generated.go`

### 5. (Optional) Add Pre-release Version for Next Major

To demonstrate version evolution, add `v2alpha1` (pre-release for v2).

Use the CLI to add the version and copy types:

```bash
# Add v2alpha1 version
fabrica add version v2alpha1

# Add Device to the new version
fabrica add resource Device --version v2alpha1
```

This updates `apis.yaml`, creates `apis/infra.example.io/v2alpha1/device_types.go`, and lets you evolve v2 changes.

Add v2beta1 to demonstrate progression from alpha → beta:

```bash
fabrica add version v2beta1 --from v2alpha1
```

When ready, create stable v2 (requires `--force` for stable versions):

```bash
fabrica add version v2 --from v2beta1 --force
fabrica add resource Device --version v2 --force
```

Then update `apis.yaml` to promote v2 as the storage hub:

```yaml
groups:
  - name: infra.example.io
    storageVersion: v2  # Change hub to v2
    versions:
      - v1              # Keep for backward compatibility
      - v2alpha1         # Can be removed once v2 is stable
      - v2beta1          # Can be removed once v2 is stable
      - v2               # Stable v2
```

After this change, run `fabrica generate` to update:
- Handlers in `cmd/server/*_handlers_generated.go`
- Storage layer in `internal/storage/storage_generated.go`
- Routes in `cmd/server/routes_generated.go`
- Client library in `pkg/client/client_generated.go`
- OpenAPI spec in `cmd/server/openapi_generated.go`
- Resource registration in `apis/infra.example.io/v2/register_generated.go`

Generated registration is written to the hub version package:

```go
// Code generated by fabrica. DO NOT EDIT.
package v2

import (
    "fmt"
    "github.com/openchami/fabrica/pkg/codegen"
)

func RegisterAllResources(gen *codegen.Generator) error {
    if err := gen.RegisterResource(&Device{}); err != nil {
        return fmt.Errorf("failed to register Device: %w", err)
    }
    return nil
}
```

### 6. Run the Server

```bash
go run ./cmd/server
```

The server starts on `http://localhost:8080`.

### 7. Test the API

Version requests follow this precedence:
1. `apiVersion` in the request body
2. Explicit URL version (e.g., `/apis/<group>/<version>/...`)
3. `Accept` header
4. Default storage version

If a version is not listed in `apis.yaml`, the server responds with `406 Not Acceptable`.

#### Create a Device

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1",
    "kind": "Device",
    "metadata": {"name": "device-1"},
    "spec": {
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
curl http://localhost:8080/devices/${device_uid}
```

> [!TIP]
> To get a device, you will need the `device-id` from the list returned using the `curl` above. Replace that value for the `device_uid` variable below.

#### Update a Device

```bash
curl -X PUT http://localhost:8080/devices/${device_uid} \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1",
    "kind": "Device",
    "metadata": {"name": "device-1"},
    "spec": {
      "ipAddress": "192.168.1.101",
      "location": "DataCenter B",
      "deviceType": "switch",
      "tags": {"env": "staging"}
    }
  }'
```

#### Delete a Device

```bash
curl -X DELETE http://localhost:8080/devices/${device_uid}
```

## Configuration Reference

### apis.yaml

```yaml
project:
  name: device-api
  module: github.com/example/device-api
  description: Device management API
  created: "2025-11-12T12:00:00Z"

groups:
  - name: infra.example.io
    storageVersion: v1
    versions:
      - v1alpha1
      - v1beta1
      - v1
    resources:
      - Device
```

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
    strategy: header

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
1. Auto-selects the storageVersion hub (e.g., `v1`)
2. This ensures resources are added to the canonical storage version by default

```bash
# Auto-selects storage hub (v1 in this example)
fabrica add resource Device

# Explicitly specify a pre-release version if needed
fabrica add resource Device --version v2alpha1
```

### Storage Version (Hub)

The `storageVersion` field defines which version is used for persistence:
- All data is stored in this format
- Should be a stable version (e.g., `v1`, not `v1alpha1`)
- Must be in the `versions` list

### Version Iteration Workflow

1. **Alpha**: Start with `v1alpha1`, iterate rapidly
2. **Beta**: Copy to `v1beta1` when semi-stable, refine schema
3. **Stable**: Copy to `v1` when ready for production, mark as `storageVersion`
4. **Deprecation**: Remove old versions from `versions` list when no longer supported

## Comparison: Versioned vs Legacy Mode

### Versioned Mode (This Example)

```bash
fabrica init device-api --group infra.example.io --versions v1alpha1,v1
```

**Structure:**
```
device-api/
├── .fabrica.yaml               # Feature flags
├── apis.yaml                   # API group/version registry
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

### Legacy Mode (Deprecated)

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
- Older projects created before hub/spoke versioning
- Migrations that have not moved to `apis.yaml`
- New projects should use versioned mode instead

## Troubleshooting

### Error: "version X not found in apis.yaml"

**Cause**: Specified version doesn't exist in config.

**Solution**: Add version to `apis.yaml` or use existing version:
```yaml
groups:
  - name: infra.example.io
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

### Validation Error: "Undefined validation function 'X'"

**Cause**: Confusion between JSON field names and validator function names in struct tags.

**Understanding validation tags**:
- The `validate` tag specifies **validator function names** (e.g., `ip`, `email`, `uuid`)
- The `json` tag specifies the **field name** in JSON requests

**Example**:
```go
type DeviceSpec struct {
    IPAddress  string `json:"ipAddress" validate:"required,ip"`
    //                       ^^^^^^^^^^^ (JSON name)    ^^ (validator function)
    Email      string `json:"email" validate:"required,email"`
    DeviceType string `json:"deviceType" validate:"oneof=server switch router"`
}
```

**Common validators**: `ip`, `email`, `uuid`, `required`, `oneof=a b c`, `min`, `max`

**In JSON requests**, use the `json` tag value:
```json
{
  "ipAddress": "192.168.1.100",
  "email": "admin@example.com",
  "deviceType": "server"
}
```

See the [Validation Guide](../../docs/guides/validation.md) for all available validators.

### Error: "No resources found"

**Cause**: Hub version directory is empty.

**Solution**: Add resource to hub (storage) version:
```bash
fabrica add resource Device --version v1 --force
```

### Generator Shows "Legacy mode"

**Cause**: `apis.yaml` is missing or versioning is disabled in `.fabrica.yaml`.

**Solution**: Ensure `apis.yaml` exists (run `fabrica init`), then enable versioning:
```yaml
features:
  versioning:
    enabled: true
    strategy: header
```

Also verify `apis.yaml` includes your versions and resources:
```yaml
groups:
  - name: infra.example.io
    storageVersion: v1
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
