<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Example 8 Structure

This example demonstrates the APIs-first versioned architecture.

## Directory Structure

```
08-api-versioning/
├── .fabrica.yaml                    # Unified configuration
├── README.md                         # Tutorial walkthrough
├── apis/
│   └── infra.example.io/
│       ├── v1alpha1/
│       │   └── device_types.go      # Alpha version (basic fields)
│       ├── v1beta1/
│       │   └── device_types.go      # Beta version (adds Tags, Conditions)
│       └── v1/
│           └── device_types.go      # Stable/hub version (complete schema)
├── cmd/
│   └── server/
│       └── main.go                  # Server template
├── go.mod                            # Go module
└── internal/
    └── storage/
        └── storage.go               # Storage stub
```

## API Version Evolution

### v1alpha1 (Alpha)
- Basic Device fields: Name, IPAddress, Location, DeviceType
- Simple status: Phase, Message, Ready, Health, LastChecked

### v1beta1 (Beta)
**Added:**
- `Spec.Tags` - map[string]string for labels
- `Status.Conditions` - []Condition for detailed status

### v1 (Stable/Hub)
- Same schema as v1beta1
- This is the storage version (hub)
- All data persisted in this format

## Key Files

### .fabrica.yaml
Shows the unified configuration with:
- Single config file (no separate apis.yaml)
- Versioning configuration with group, storage_version, versions, resources
- All feature flags in one place

### apis/infra.example.io/*/device_types.go
Shows the flattened envelope structure:
- Imports `fabrica.Metadata` (clean import path)
- Explicit APIVersion, Kind, Metadata fields
- No embedding of resource.Resource
- Package name = version (v1alpha1, v1beta1, v1)

## Running the Example

This is a structural example showing the directory layout and type definitions.
To make it functional:

```bash
cd examples/08-api-versioning

# Generate handlers, storage, client, etc.
fabrica generate

# Tidy dependencies
go mod tidy

# Run server
go run ./cmd/server
```

After generation, the following will be created:
- `pkg/handlers/device/` - HTTP handlers
- `pkg/storage/` - Storage interface
- `pkg/client/` - Go client
- `pkg/resources/register_generated.go` - Resource registration
- OpenAPI spec

## What This Demonstrates

1. **No Redundancy**: Types defined once per version, not in both pkg/resources/ and apis/
2. **Clean Imports**: Uses `fabrica.Metadata` instead of `resource.Metadata`
3. **Version Evolution**: Shows how to evolve API schema from alpha → beta → stable
4. **Unified Config**: Single .fabrica.yaml instead of multiple config files
5. **Flattened Envelope**: Explicit fields instead of embedding
