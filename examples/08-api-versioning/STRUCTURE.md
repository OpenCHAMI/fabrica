<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Example 8 Structure

This example demonstrates the APIs-first versioned architecture.

## Directory Structure

```
08-api-versioning/
├── .fabrica.yaml                    # Feature flags
├── apis.yaml                        # API group/version registry
├── README.md                         # Tutorial walkthrough
├── go.mod                            # Go module
├── cmd/
│   └── server/
│       ├── main.go                  # Server entry point
│       ├── *_handlers_generated.go  # Generated handlers
│       ├── routes_generated.go      # Generated routes
│       └── models_generated.go      # Generated models
├── apis/
│   └── infra.example.io/
│       ├── v1/
│       │   ├── device_types.go      # Stable/hub version (storage)
│       │   └── register_generated.go # Resource registration
│       ├── v2alpha1/                # Optional: v2 pre-release
│       │   └── device_types.go      # Alpha version for v2
│       └── v2beta1/                 # Optional: v2 beta
│           └── device_types.go      # Beta version for v2
├── internal/
│   ├── middleware/
│   │   └── *_middleware_generated.go
│   └── storage/
│       ├── storage.go               # Storage interface
│       └── storage_generated.go     # Generated storage impl
└── pkg/
    └── client/
        └── client_generated.go      # Generated Go client
```

## API Version Evolution

### v1 (Stable/Hub)
- Current stable API version
- **Storage version**: All data persisted in this format
- Complete Device schema with IPAddress, Location, DeviceType, Tags, Description
- Status includes Phase, Message, Ready, LastChecked, Conditions

### v2alpha1 (Optional - Alpha for next major)
- Pre-release version for v2 API
- Allows testing breaking changes before v2 stable
- Can have experimental features not in v1

### v2beta1 (Optional - Beta for next major)
- Refined v2 API approaching stability
- Feature-complete for v2, may have minor changes
- Demonstrates progression: v2alpha1 → v2beta1 → v2

### Version Strategy
- **v1** is the current production API (hub/storage)
- **v2alpha1, v2beta1** demonstrate evolution toward next major
- Pre-releases (alpha/beta) belong to the *next* major version
- When v2 is stable, it becomes the new hub/storage version

## Key Files

### apis.yaml
Shows the versioning registry with:
- API group name
- `storageVersion` for the hub
- Versions list and resource inventory

### .fabrica.yaml
Shows feature flags and generator settings (validation, storage, middleware)

### apis/infra.example.io/*/device_types.go
Shows the flattened envelope structure:
- Imports `fabrica.Metadata` (clean import path)
- Explicit APIVersion, Kind, Metadata fields
- No embedding of resource.Resource
- Package name = version (v1alpha1, v1beta1, v1)

## Running the Example

This is a structural example. To create a working version:

```bash
# Initialize with versioning
fabrica init my-device-api \
  --module github.com/user/my-device-api \
  --group infra.example.io \
  --storage-version v1

cd my-device-api

# Add a resource
fabrica add resource Device

# Generate code
fabrica generate
go mod tidy

# Run server
go run ./cmd/server/
```

After generation, the following will be created:
- `cmd/server/*_handlers_generated.go` - HTTP handlers
- `internal/storage/storage_generated.go` - Storage implementation
- `pkg/client/client_generated.go` - Go client
- `apis/infra.example.io/v1/register_generated.go` - Resource registration
- `cmd/server/openapi_generated.go` - OpenAPI specification
- `cmd/server/routes_generated.go` - Route registration

## What This Demonstrates

1. **No Redundancy**: Types defined once per version, not duplicated across folders
2. **Clean Imports**: Uses `fabrica.Metadata` instead of `resource.Metadata`
3. **Version Evolution**: Shows how to evolve API schema from alpha → beta → stable
4. **Split Config**: `apis.yaml` for versions, `.fabrica.yaml` for feature flags
5. **Flattened Envelope**: Explicit fields instead of embedding
