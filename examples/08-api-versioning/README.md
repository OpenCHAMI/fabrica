<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 8: Hub/Spoke API Versioning

This example demonstrates Fabrica's Kubebuilder-style hub/spoke API versioning system. You'll learn how to:

- Define multiple API versions (v1alpha1, v1beta1, v1)
- Use a single hub (storage) version with multiple spoke (external) versions
- Automatically convert between versions
- Request specific API versions from clients

## What This Example Shows

**Hub/Spoke Versioning**: Fabrica generates:
- One **hub** version (`v1`) used for storage
- Multiple **spoke** versions (`v1alpha1`, `v1beta1`, `v1`) exposed to clients
- Automatic conversion between hub and spokes

**Version Negotiation**: Clients can request specific versions:
```bash
# Request v1beta1
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{"apiVersion": "infra.example.io/v1beta1", ...}'

# Request v1 (stable)
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{"apiVersion": "infra.example.io/v1", ...}'
```

## Scenario: Device Management API

We're building a device management API that needs to support:
- **v1alpha1**: Early alpha version with basic fields
- **v1beta1**: Beta version with additional metadata
- **v1**: Stable version (also the hub/storage version)

## Prerequisites

- Fabrica installed (see main README)
- Go 1.21 or later

## Project Structure

```
08-api-versioning/
├── README.md (this file)
├── apis.yaml                    # API group/version configuration
├── go.mod
├── cmd/
│   └── server/
│       └── main.go              # Server with version middleware
├── pkg/
│   └── resources/
│       └── device/
│           └── device.go        # Device resource definition
└── apis/                        # Generated versioned types
    └── infra.example.io/
        ├── v1/                  # Hub (storage version)
        │   ├── types_generated.go
        │   └── register_generated.go
        ├── v1beta1/             # Spoke (external version)
        │   ├── types_generated.go
        │   └── conversions_generated.go
        └── v1alpha1/            # Spoke (external version)
            ├── types_generated.go
            └── conversions_generated.go
```

## Step-by-Step Guide

### 1. Initialize the Project

```bash
# Create project
mkdir device-api-versioned
cd device-api-versioned

fabrica init device-api-versioned
```

### 2. Define Your Resource

```bash
fabrica add resource Device
```

Edit `pkg/resources/device/device.go`:

```go
package device

import "github.com/openchami/fabrica/pkg/resource"

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status"`
}

type DeviceSpec struct {
    Name        string `json:"name" validate:"required"`
    IPAddress   string `json:"ipAddress" validate:"required,ip"`
    Location    string `json:"location"`
    DeviceType  string `json:"deviceType" validate:"oneof=server switch router"`
}

type DeviceStatus struct {
    Health      string `json:"health" validate:"oneof=healthy degraded unhealthy"`
    LastChecked string `json:"lastChecked"`
}

func init() {
    resource.RegisterResourcePrefix("Device", "dev")
}
```

### 3. Configure API Versions

Create `apis.yaml` in the project root:

```yaml
groups:
  - name: infra.example.io
    storageVersion: v1
    versions:
      - v1alpha1
      - v1beta1
      - v1
    resources:
      - kind: Device
        # Optional: specify version-specific field mappings
        # This example uses 1:1 field mappings (default)
```

### 4. Generate Code

```bash
fabrica generate
go mod tidy
```

This generates:
- Hub version in `apis/infra.example.io/v1/`
- Spoke versions in `apis/infra.example.io/v1alpha1/` and `v1beta1/`
- Conversion functions
- Version registry and middleware

### 5. Run the Server

```bash
go run ./cmd/server
```

The server starts on `http://localhost:8080` with version negotiation enabled.

### 6. Test Version Negotiation

#### Create a Device with v1beta1

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1beta1",
    "kind": "Device",
    "metadata": {"name": "device-beta"},
    "spec": {
      "name": "device-beta",
      "ipAddress": "192.168.1.100",
      "location": "DataCenter A",
      "deviceType": "server"
    }
  }'
```

#### Create a Device with v1 (stable)

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "infra.example.io/v1",
    "kind": "Device",
    "metadata": {"name": "device-stable"},
    "spec": {
      "name": "device-stable",
      "ipAddress": "192.168.1.101",
      "location": "DataCenter B",
      "deviceType": "switch"
    }
  }'
```

#### List All Devices (returns preferred version)

```bash
curl http://localhost:8080/devices
```

#### Request Specific Version via Accept Header

```bash
curl -X GET http://localhost:8080/devices/device-beta \
  -H "Accept: application/json; api-version=infra.example.io/v1beta1"
```

## How It Works

### 1. Version Negotiation Middleware

When a request arrives:
1. Middleware reads `apiVersion` from request body (or Accept header)
2. Decodes into the requested spoke version (e.g., `v1beta1.Device`)
3. Converts to hub version (`v1.Device`) via `ConvertTo()`
4. Handler/storage operates on hub version

When a response is sent:
1. Handler returns hub version
2. Middleware converts to requested spoke version via `ConvertFrom()`
3. Encodes and sends response

### 2. Flattened Envelope

Generated types use explicit fields instead of embedding `resource.Resource`:

```go
// Generated hub type (apis/infra.example.io/v1/types_generated.go)
type Device struct {
    APIVersion string       `json:"apiVersion"` // "infra.example.io/v1"
    Kind       string       `json:"kind"`       // "Device"
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}

func (Device) IsHub() {} // Marker for hub version
```

### 3. Automatic Conversions

Spoke versions implement `ConvertTo()` and `ConvertFrom()`:

```go
// Generated spoke type (apis/infra.example.io/v1beta1/types_generated.go)
func (src *Device) ConvertTo(dstRaw interface{}) error {
    dst := dstRaw.(*v1.Device)

    // Copy metadata
    dst.Metadata = v1.Metadata{
        Name: src.Metadata.Name,
        UID:  src.Metadata.UID,
        // ...
    }

    // Copy spec fields (1:1 mapping by default)
    dst.Spec = v1.DeviceSpec{
        Name:       src.Spec.Name,
        IPAddress:  src.Spec.IPAddress,
        Location:   src.Spec.Location,
        DeviceType: src.Spec.DeviceType,
    }

    return nil
}
```

## Key Concepts

### Hub vs Spoke

- **Hub (`v1`)**: Storage version. All data is persisted in this format.
- **Spokes (`v1alpha1`, `v1beta1`, `v1`)**: External versions. Clients can request any spoke.

### Storage Stability

Since the hub is stable, you can add/remove spoke versions without migrating storage data.

### Version Lifecycle

1. **v1alpha1**: Experimental features, may change without notice
2. **v1beta1**: Semi-stable, breaking changes announced in advance
3. **v1**: Stable, follows semantic versioning

## Troubleshooting

### Error: "apiVersion not supported"

**Cause**: Requested version not in `apis.yaml`.

**Solution**: Add version to the `versions:` list in `apis.yaml` and regenerate.

### Error: "Conversion failed"

**Cause**: Field mismatch between hub and spoke.

**Solution**: Edit conversion functions in `apis/<group>/<version>/conversions_generated.go`.

### Storage Shows Different Version

**Cause**: Storage always uses the hub version.

**Solution**: This is expected. Clients see the spoke version they requested, but storage uses the hub.

## Next Steps

- **Add Breaking Changes**: Create a new spoke version (`v2beta1`) with field renames
- **Custom Conversions**: Edit generated conversion functions for complex mappings
- **Deprecation**: Remove old spoke versions from `apis.yaml` after announcing deprecation
- **External Types**: Use the `imports:` section in `apis.yaml` to reference types from other packages

## Learn More

- [Hub/Spoke Versioning Guide](../../docs/versioning.md)
- [Kubebuilder Versioning](https://book.kubebuilder.io/multiversion-tutorial/tutorial.html)
- [Getting Started](../../docs/guides/getting-started.md)
