<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Hub/Spoke API Versioning in Fabrica

Fabrica implements **Kubebuilder-style hub/spoke versioning** to provide stable APIs while allowing evolution of your resource schemas over time. This guide explains the versioning model, how to use it, and how to migrate existing code.

## Table of Contents

- [Why Hub/Spoke Versioning?](#why-hubspoke-versioning)
- [Versioning Model](#versioning-model)
- [Quick Start](#quick-start)
- [Declaring API Groups and Versions](#declaring-api-groups-and-versions)
- [Requesting a Specific Version](#requesting-a-specific-version)
- [Generated Code Structure](#generated-code-structure)
- [Conversion Mechanism](#conversion-mechanism)
- [Breaking Changes and Migration](#breaking-changes-and-migration)
- [Migration from Pre-Flattening](#migration-from-pre-flattening)
- [Best Practices](#best-practices)

## Why Hub/Spoke Versioning?

**Storage Stability**: Your internal storage format (the "hub") remains stable, while external APIs (the "spokes") can evolve independently. This allows you to:

- Add new API versions without migrating stored data
- Deprecate old versions gracefully
- Support multiple client versions simultaneously

**Client Stability**: Clients can pin to a specific API version and continue working even as you add new features to newer versions.

**Safe Evolution**: Breaking changes to your types can be introduced in a new spoke version while the hub remains unchanged.

## Versioning Model

```
┌─────────────────────────────────────────┐
│           Client Request                │
│   (apiVersion: infra.example.io/v1beta1)│
└──────────────────┬──────────────────────┘
                   │
                   ▼
         ┌─────────────────┐
         │ Version Middleware│
         │  (negotiation)   │
         └────────┬──────────┘
                  │
                  ▼ Convert to Hub
         ┌────────────────┐
         │   Hub (v1)     │  ◄── Storage always uses this
         │ Storage Version │
         └────────┬────────┘
                  │
                  ▼ Convert to Requested Spoke
         ┌─────────────────┐
         │  Spoke (v1beta1) │
         │  Response        │
         └──────────────────┘
```

- **Hub**: The storage version (`v1`). All resources are stored in this format.
- **Spokes**: External API versions (`v1alpha1`, `v1beta1`, `v1`). Clients can request any spoke version.
- **Conversions**: Automatic translation between hub and spokes via generated functions.

## Quick Start

By default, Fabrica generates resources with a single version (`v1`) that acts as both hub and spoke. To enable multi-version support:

### 1. Create an `apis.yaml` in your generated project:

```yaml
groups:
  - name: infra.example.io
    storageVersion: v1
    versions:
      - v1alpha1
      - v1beta1
      - v1
    imports:
      - module: github.com/yourorg/netmodel
        tag: v0.9.3
        packages:
          - path: api/types
            expose:
              - kind: Device
                specFrom: github.com/yourorg/netmodel/api/types.DeviceSpec
                statusFrom: github.com/yourorg/netmodel/api/types.DeviceStatus
```

### 2. Run `fabrica generate`:

```bash
fabrica generate
```

This will generate:

- `apis/infra.example.io/v1/types_generated.go` (hub)
- `apis/infra.example.io/v1beta1/types_generated.go` (spoke)
- `apis/infra.example.io/v1alpha1/types_generated.go` (spoke)
- Conversion functions between hub and spokes
- Version registry and middleware

### 3. Resources now have explicit flattened envelopes:

```go
// OLD (pre-flattening):
type Device struct {
    resource.Resource[DeviceSpec, DeviceStatus]
}

// NEW (flattened):
type Device struct {
    APIVersion string       `json:"apiVersion"`
    Kind       string       `json:"kind"`
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}
```

**Note**: The JSON wire format remains identical; only the Go struct shape changes.

## Declaring API Groups and Versions

The `apis.yaml` file defines your API groups and versions:

```yaml
groups:
  - name: infra.example.io         # API group name
    storageVersion: v1              # Hub version (used for storage)
    versions:                       # Spoke versions (external APIs)
      - v1alpha1                    # Alpha version (unstable)
      - v1beta1                     # Beta version (semi-stable)
      - v1                          # Stable version
    imports:                        # Optional: import external types
      - module: github.com/org/pkg
        tag: v1.0.0
        packages:
          - path: api/types
            expose:
              - kind: MyResource
                specFrom: pkg.MyResourceSpec
                statusFrom: pkg.MyResourceStatus
```

### Version Stability Levels

- **`v1alpha1`, `v1alpha2`**: Alpha versions. Unstable, may change without notice.
- **`v1beta1`, `v1beta2`**: Beta versions. Semi-stable, breaking changes announced in advance.
- **`v1`, `v2`**: Stable versions. Changes follow semantic versioning.

## Requesting a Specific Version

Clients can request a specific version using the `apiVersion` field in the request body:

### Via API Version Field (Recommended)

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1beta1",
    "kind": "Device",
    "metadata": {"name": "device-01"},
    "spec": { ... }
  }'
```

### Via Accept Header (Alternative)

```bash
curl -X GET http://localhost:8080/devices/device-01 \
  -H "Accept: application/json; api-version=infra.example.io/v1beta1"
```

If no version is specified, the server returns the **preferred version** (typically the storage version).

## Generated Code Structure

With versioning enabled, Fabrica generates:

```
apis/
└── infra.example.io/
    ├── v1/                          # Hub (storage version)
    │   ├── types_generated.go       # Flattened Device type
    │   └── register_generated.go
    ├── v1beta1/                     # Spoke (external version)
    │   ├── types_generated.go       # Flattened Device type
    │   └── conversions_generated.go # Conversion to/from hub
    └── v1alpha1/                    # Spoke (external version)
        ├── types_generated.go
        └── conversions_generated.go
```

### Hub Type Example (`apis/infra.example.io/v1/types_generated.go`)

```go
package v1

type Device struct {
    APIVersion string       `json:"apiVersion"` // "infra.example.io/v1"
    Kind       string       `json:"kind"`       // "Device"
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}

// IsHub marks this as the hub version
func (Device) IsHub() {}
```

### Spoke Type Example (`apis/infra.example.io/v1beta1/types_generated.go`)

```go
package v1beta1

import v1 "yourmodule/apis/infra.example.io/v1"

type Device struct {
    APIVersion string       `json:"apiVersion"` // "infra.example.io/v1beta1"
    Kind       string       `json:"kind"`
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}

// ConvertTo converts this spoke to the hub
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)
    // Field-by-field conversion logic...
    return nil
}

// ConvertFrom converts from the hub to this spoke
func (dst *Device) ConvertFrom(srcRaw interface{}) error {
    src := srcRaw.(*v1.Device)
    // Field-by-field conversion logic...
    return nil
}
```

## Conversion Mechanism

Conversions between hub and spokes happen automatically via middleware:

1. **Incoming Request**:
   - Client sends `apiVersion: infra.example.io/v1beta1`
   - Middleware decodes into `v1beta1.Device`
   - Converts to `v1.Device` (hub) via `ConvertTo()`
   - Handler/storage operates on hub version

2. **Outgoing Response**:
   - Handler returns `v1.Device` (hub)
   - Middleware converts to `v1beta1.Device` via `ConvertFrom()`
   - Response sent to client as `v1beta1`

### Custom Conversions

For complex field transformations, edit the generated conversion functions:

```go
// Custom conversion for renamed field
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)

    // Standard field copy
    dst.Spec.Name = src.Spec.Name

    // Custom transformation: v1beta1 "ipAddress" → v1 "ip"
    dst.Spec.IP = src.Spec.IPAddress

    return nil
}
```

## Breaking Changes and Migration

When making breaking changes to your types:

### Option 1: Add a New Spoke Version

1. Keep the hub (`v1`) unchanged
2. Add a new spoke (`v2beta1`) with the breaking change
3. Implement custom conversion logic
4. Deprecate the old spoke version

**Example**: Renaming a field

```yaml
# apis.yaml
groups:
  - name: infra.example.io
    storageVersion: v1
    versions:
      - v1alpha1        # Old version
      - v1beta1         # Current version
      - v2beta1         # New version with breaking change
      - v1              # Stable
```

### Option 2: Bump the Hub (Major Version Bump)

When the hub needs to change (e.g., removing deprecated fields):

1. Create a new hub version (`v2`)
2. Migrate storage data from `v1` to `v2`
3. Update spokes to convert to/from `v2`

## Migration from Pre-Flattening

If you have existing Fabrica code using `resource.Resource[Spec, Status]`:

### Before (Embedded Generic)

```go
type Device struct {
    resource.Resource[DeviceSpec, DeviceStatus]
}
```

### After (Flattened Envelope)

```go
type Device struct {
    APIVersion string       `json:"apiVersion"`
    Kind       string       `json:"kind"`
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}
```

### Migration Steps

1. **Regenerate Code**: Run `fabrica generate` to get flattened types
2. **Update Custom Code**: If you have custom handlers or reconcilers that reference the embedded `Resource` field, update them:

```go
// OLD:
device.Resource.Metadata.Name

// NEW:
device.Metadata.Name
```

3. **JSON Compatibility**: The JSON wire format is **unchanged**, so existing clients and data continue to work.

## Best Practices

### 1. Start with a Single Version

For new projects, start with `v1` only. Add spokes (`v1alpha1`, `v1beta1`) only when you need to experiment with breaking changes.

### 2. Use Semantic Versioning

- `v1alpha*`: Experimental features, may change
- `v1beta*`: Semi-stable features, breaking changes announced
- `v1`, `v2`: Stable, follows semantic versioning

### 3. Keep the Hub Stable

Minimize changes to the hub version. Use spokes for experimentation and deprecation.

### 4. Document Version Differences

In your API documentation, clearly describe differences between versions:

```markdown
## API Versions

- **v1**: Stable. Recommended for production.
- **v1beta1**: Beta. Includes experimental field `advancedOptions`.
- **v1alpha1**: Alpha. For testing only.
```

### 5. Deprecation Policy

When deprecating a version:

1. Announce deprecation in release notes
2. Keep the version available for at least 2-3 minor releases
3. Remove in the next major version

---

## Troubleshooting

### Error: "apiVersion not supported"

**Cause**: Client requested a version not in the `apis.yaml` spokes list.

**Solution**: Add the version to `apis.yaml` or update the client to use a supported version.

### Error: "Conversion failed"

**Cause**: Field mismatch between hub and spoke (e.g., renamed field).

**Solution**: Implement custom conversion logic in the generated `ConvertTo`/`ConvertFrom` functions.

### JSON Format Changed

**Cause**: Regeneration flattened the envelope, but custom code expects the old embedded `Resource` field.

**Solution**: Update custom code to use `device.Metadata.Name` instead of `device.Resource.Metadata.Name`.

---

## See Also

- [Resource Model Guide](guides/resource-model.md)
- [Getting Started](guides/getting-started.md)
- [Storage Systems](guides/storage.md)
