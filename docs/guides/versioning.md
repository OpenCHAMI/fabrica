<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# API Versioning Guide

> Hub/spoke API versioning with automatic conversion, storage stability, and smooth client migrations.

## Table of Contents

- [Overview](#overview)
- [Why Hub/Spoke Versioning](#why-hubspoke-versioning)
- [Versioning Model](#versioning-model)
- [Quick Start](#quick-start)
- [Version Registration](#version-registration)
- [Conversion Patterns](#conversion-patterns)
- [HTTP Negotiation](#http-negotiation)
- [Migration Strategies](#migration-strategies)
- [Breaking Changes](#breaking-changes-and-migration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Fabrica implements **Kubebuilder-style hub/spoke versioning** to provide stable APIs while allowing evolution of your resource schemas over time. This allows you to:

- Support multiple API versions simultaneously
- Evolve schemas without breaking clients
- Maintain storage stability independent of API versions
- Perform automatic version conversion via middleware
- Enable smooth client migrations with gradual deprecation

### Hub vs. Spoke

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

- **Hub**: The storage version (typically `v1`). All resources are stored in this format. All internal operations use the hub version.
- **Spokes**: External API versions (`v1alpha1`, `v1beta1`, `v1`, etc.). Clients request any spoke version and the server automatically converts.
- **Conversions**: Automatic translation between hub and spokes via generated middleware.

## Why Hub/Spoke Versioning

### Storage Stability

Your internal storage format (the "hub") remains stable while external APIs (the "spokes") evolve independently. This allows you to:

- Add new API versions without migrating stored data
- Deprecate old versions gracefully
- Support multiple client versions simultaneously

### Client Stability

Clients can pin to a specific API version and continue working even as you add new features to newer versions.

### Safe Evolution

Breaking changes to your types can be introduced in a new spoke version while the hub remains unchanged.

## Versioning Model

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

- **`v1alpha1`, `v1alpha2`**: Alpha versions. Unstable, may change without notice. Resources can be added freely.
- **`v1beta1`, `v1beta2`**: Beta versions. Semi-stable, breaking changes announced in advance. Resources can be added freely.
- **`v1`, `v2`**: Stable versions. Changes follow semantic versioning. Adding new resources requires `--force` flag.

## Quick Start

By default, Fabrica generates resources with a single version (`v1`) that acts as both hub and spoke. To enable multi-version support:

### Adding a New API Version

**Step 1: Add the version to your project**

```bash
# Add an alpha version (for experimentation)
fabrica add version v2alpha1

# Or add a stable version (requires --force)
fabrica add version v2 --force
```

This command:
- Updates `apis.yaml` with the new version
- Creates the version directory (`apis/<group>/<version>/`)
- Copies existing resource types from the latest version

**Step 2: Add or modify resources in the new version**

```bash
# Add a new resource to the alpha version
fabrica add resource Feature --version v2alpha1

# Modify existing resources in the version directory
# Edit apis/<group>/v2alpha1/device_types.go
```

**Important**: You must use `fabrica add version` before adding resources to a new version. If you try to add a resource to a non-existent version:

```bash
fabrica add resource Device --version v2
# Error: version v2 not found in apis.yaml (available: [v1])
#
# To add a new version, run: fabrica add version v2
```

**Step 3: Generate handlers and conversions**

```bash
fabrica generate
```

This generates:
- `apis/<group>/v1/types_generated.go` (hub)
- `apis/<group>/v2alpha1/types_generated.go` (spoke)
- Conversion functions between hub and spokes
- Version registry and middleware

## Version Registration

Versioning is driven by `apis.yaml` plus CLI scaffolding. No manual registration is required.

**Single version (default):**
1) `fabrica init myapi --group example.fabrica.dev --versions v1`
2) `fabrica add resource Device --version v1`
3) Edit `apis/example.fabrica.dev/v1/device_types.go`
4) `fabrica generate`

**Add another version (spoke):**
1) `fabrica add version v2beta1` (copies types into `apis/example.fabrica.dev/v2beta1/` and updates `apis.yaml`)
2) Update the v2beta1 types as needed
3) Add converters under `apis/example.fabrica.dev/v2beta1/converter.go`
4) `fabrica generate`

### Generated Code Structure

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

## Conversion Patterns

### Define Version Structs

**v1 (stable/hub):**
```go
// apis/example.fabrica.dev/v1/device_types.go
package v1

type Device struct {
    APIVersion string       `json:"apiVersion"`
    Kind       string       `json:"kind"`
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Username string `json:"username"` // Flat auth
    Password string `json:"password"`
}
```

**v2beta1 (spoke with structured auth):**
```go
// apis/example.fabrica.dev/v2beta1/device_types.go
package v2beta1

type Device struct {
    APIVersion string       `json:"apiVersion"`
    Kind       string       `json:"kind"`
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Auth     AuthConfig `json:"auth"` // Structured auth
}

type AuthConfig struct {
    Type     string `json:"type"` // "basic", "oauth", "cert"
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`
    Token    string `json:"token,omitempty"`
}
```

### Implement Converter

```go
// apis/example.fabrica.dev/v2beta1/converter.go
package v2beta1

import (
    v1 "yourmodule/apis/example.fabrica.dev/v1"
)

// ConvertTo converts v2beta1 Device to v1 (hub)
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)

    // Copy flattened envelope fields
    dst.APIVersion = src.APIVersion
    dst.Kind = src.Kind
    dst.Metadata = src.Metadata

    // Standard field copy
    dst.Spec.Name = src.Spec.Name
    dst.Spec.Location = src.Spec.Location

    // Custom transformation: v2beta1 structured auth → v1 flat auth
    if src.Spec.Auth.Type == "basic" {
        dst.Spec.Username = src.Spec.Auth.Username
        dst.Spec.Password = src.Spec.Auth.Password
    } else {
        log.Warn("Non-basic auth will be lost in v1 conversion")
    }

    dst.Status = src.Status
    return nil
}

// ConvertFrom converts v1 (hub) Device to v2beta1
func (dst *Device) ConvertFrom(srcRaw interface{}) error {
    src := srcRaw.(*v1.Device)

    // Copy flattened envelope fields
    dst.APIVersion = src.APIVersion
    dst.Kind = src.Kind
    dst.Metadata = src.Metadata

    // Standard field copy
    dst.Spec.Name = src.Spec.Name
    dst.Spec.Location = src.Spec.Location

    // Custom transformation: v1 flat auth → v2beta1 structured auth
    dst.Spec.Auth = AuthConfig{
        Type:     "basic",
        Username: src.Spec.Username,
        Password: src.Spec.Password,
    }

    dst.Status = src.Status
    return nil
}
```

### Register Converter

No manual registration is needed; the generator wires converters automatically:

1) Add your converter next to the versioned types (e.g., `apis/example.fabrica.dev/v2beta1/converter.go`).
2) Export a constructor: `func NewDeviceConverter() versioning.VersionConverter { return &DeviceConverter{} }`.
3) Run `fabrica generate`; the generator discovers and registers converters via `apis.yaml`.

## HTTP Negotiation

Clients can request a specific version using the `apiVersion` field in the request body, an explicit versioned URL, or the `Accept` header.

**Precedence (highest to lowest):**
1. `apiVersion` in request body (POST/PUT/PATCH)
2. Explicit version in URL (e.g., `/apis/<group>/<version>/...`)
3. `Accept` header (`application/json;version=v1beta1`)
4. Default/storage version

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

If no version is specified, the server returns the **preferred version** (typically the storage version). If a requested version is not registered in `apis.yaml`, the server responds with `406 Not Acceptable`.

### Conversion Flow

**Request Flow:**
1. Client sends request with `apiVersion: infra.example.io/v1beta1`
2. Middleware decodes into `v1beta1.Device` type
3. Middleware converts to `v1.Device` (hub) via `ConvertTo()`
4. Handler/storage operates on hub version

**Response Flow:**
1. Handler returns `v1.Device` (hub)
2. Middleware converts to `v1beta1.Device` via `ConvertFrom()`
3. Response sent to client as `v1beta1`

### Using curl

```bash
# Request v1
curl -H "Accept: application/json;version=v1" \
  http://localhost:8080/devices/dev-123

# Request v2beta1
curl -H "Accept: application/json;version=v2beta1" \
  http://localhost:8080/devices/dev-123

# Request default version (omit version)
curl http://localhost:8080/devices/dev-123
```

### Custom Version Negotiation

For complex negotiation strategies, you can implement custom middleware in your project:

```go
// Custom version negotiation middleware
func VersionNegotiation(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check client header, query param, or other criteria
        version := r.Header.Get("X-API-Version")
        if version == "" {
            version = r.URL.Query().Get("version")
        }
        if version == "" {
            version = "v1" // default
        }

        // Store in context
        ctx := context.WithValue(r.Context(), "version", version)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Migration Strategies

### Strategy 1: Big Bang (Not Recommended)

```
Day 1: Launch v2, deprecate v1
Day 30: Remove v1
```

**Pros:** Simple
**Cons:** Breaks clients, forces immediate migration

### Strategy 2: Gradual Migration (Recommended)

```
Day 1: Launch v2beta1 for testing
Day 30: Promote to v2 stable
Day 60: Mark v1 as deprecated
Day 180: Remove v1 support
```

**Pros:** Smooth transition, no breakage
**Cons:** More work, temporary maintenance burden

### Strategy 3: Parallel Versions

```
Support v1, v2, v3 indefinitely
```

**Pros:** Maximum compatibility
**Cons:** Significant maintenance burden

### Implementation Timeline Example

**Phase 1: Launch Beta (Day 1)**
```bash
fabrica add version v2beta1
# Edit apis/example.fabrica.dev/v2beta1/*_types.go and converter
fabrica generate
```

**Phase 2: Promote to Stable (Day 30)**
```bash
# Update converters if needed
fabrica generate
```

**Phase 3: Deprecate Old Version (Day 60)**
```yaml
# Communication to clients: "v1 deprecated; migrate to v2 by Day 180"
```

**Phase 4: Remove Old Version (Day 180)**
```yaml
# Remove v1 from apis.yaml
# Update all links and documentation
fabrica generate
```

## Breaking Changes and Migration

When making breaking changes to your types, you have two options:

### Option 1: Add a New Spoke Version (Recommended)

1. Keep the hub (`v1`) unchanged
2. Add a new spoke (`v2beta1`) with the breaking change
3. Implement custom conversion logic to handle field changes
4. Deprecate the old spoke version gradually

**Example**: Renaming a field

```yaml
# apis.yaml
groups:
  - name: infra.example.io
    storageVersion: v1              # Hub stays unchanged
    versions:
      - v1alpha1                     # Old version
      - v1beta1                      # Current version
      - v2beta1                      # New version with breaking change
      - v1                           # Stable
```

**Field renaming conversion:**

```go
// apis/example.fabrica.dev/v2beta1/converter.go
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)

    // Custom transformation: v2beta1 "hostname" → v1 "ipAddress"
    dst.Spec.IPAddress = src.Spec.Hostname

    return nil
}
```

### Option 2: Bump the Hub (Major Version Bump)

When the hub itself needs to change (rare), and you want to remove deprecated fields:

1. Create a new hub version (`v2`)
2. Migrate storage data from `v1` to `v2`
3. Update old spoke versions to convert to/from `v2`
4. Deprecate the old hub

**Note**: This is complex and should be avoided in most cases. Prefer keeping the hub stable and using spokes.

## Best Practices

### Version Design

**DO:**
```go
✅ Use semantic versioning (v1, v2, v3)
✅ Mark stability (alpha, beta, stable)
✅ Provide bidirectional conversion
✅ Document breaking changes in release notes
✅ Give deprecation warnings to clients
✅ Test all conversion paths thoroughly
```

**DON'T:**
```go
❌ Use arbitrary version strings
❌ Break existing versions without warning
❌ Skip alpha/beta for major changes
❌ Remove versions without deprecation period
❌ Forget to update converters when changing fields
```

### Conversion Best Practices

**DO:**
```go
✅ Handle all field mappings
✅ Document lossy conversions (data loss warnings)
✅ Provide sensible default values for missing fields
✅ Test all conversion paths (v1→v2 and v2→v1)
✅ Log conversion warnings for complex transformations
```

**Example:**
```go
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)

    // Safe field copy
    dst.Spec.Name = src.Spec.Name

    // Lossy conversion with warning
    if src.Spec.Auth.Type != "basic" {
        log.Warn("Non-basic auth will be lost in v1 conversion")
    }

    // Provide default value
    if src.Spec.Timeout == 0 {
        dst.Spec.Timeout = 30 * time.Second
    } else {
        dst.Spec.Timeout = src.Spec.Timeout
    }

    return nil
}
```

### Migration Communication

**DO:**
```
✅ Announce deprecation in release notes
✅ Provide migration guides with examples
✅ Support multiple versions during transition (3-6 months minimum)
✅ Include links to migration documentation in error messages
✅ Monitor version usage metrics
```

**Example deprecation notice:**
```
## Deprecation Notice (v1.5.0)

The Device API v1 is deprecated and will be removed in v2.0 (Q4 2025).

**Action required:** Migrate to Device API v2 by October 31, 2025.

See [Migration Guide](https://docs.example.io/migration) for step-by-step instructions.
```

## Troubleshooting

### Error: "apiVersion not supported"

**Cause**: Client requested a version not in the `apis.yaml` versions list, or
the version registry was not generated/imported in the server.

**Solution**: Add the version to `apis.yaml` or update the client to use a supported version.
If the registry is missing, run `fabrica generate` and ensure
`_ "<module>/pkg/apiversion"` is imported in `cmd/server/main.go`.

```yaml
# apis.yaml
groups:
  - name: infra.example.io
    versions:
      - v1         # Add missing version here
      - v2beta1
```

### Error: "Conversion failed"

**Cause**: Field mismatch between hub and spoke (e.g., renamed field without conversion logic).

**Solution**: Implement custom conversion logic in the generated `ConvertTo`/`ConvertFrom` functions.

```go
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)
    // Ensure all fields are mapped
    dst.Spec.NewFieldName = src.Spec.OldFieldName
    return nil
}
```

### Error: "ConvertFrom not implemented"

**Cause**: Spoke type is missing the conversion handler.

**Solution**: Ensure the converter is in the correct package and is exported.

```go
// apis/example.fabrica.dev/v2beta1/converter.go
func (dst *Device) ConvertFrom(srcRaw interface{}) error {
    src := srcRaw.(*v1.Device)
    // Implement reverse conversion...
    return nil
}
```

### Clients receiving unexpected format

**Cause**: Conversion middleware not configured or client not specifying version.

**Solution**: Ensure:
1. Converters are properly generated (`fabrica generate`)
2. Client is sending `apiVersion` in request
3. Version middleware is registered in server setup

## See Also

- [Resource Model Guide](resource-model.md)
- [Getting Started](getting-started.md)
- [API YAML Configuration](../apis-yaml.md)
- [Quickstart Example](quickstart.md)
