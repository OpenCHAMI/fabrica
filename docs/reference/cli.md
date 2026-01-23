<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica CLI Reference

> Complete command-line reference for the Fabrica code generator.

## Table of Contents

- [Overview](#overview)
- [Global Options](#global-options)
- [Commands](#commands)
  - [fabrica init](#fabrica-init)
  - [fabrica add resource](#fabrica-add-resource)
  - [fabrica add version](#fabrica-add-version)
  - [fabrica generate](#fabrica-generate)
  - [fabrica ent generate](#fabrica-ent-generate)
  - [fabrica version](#fabrica-version)
- [Configuration Files](#configuration-files)
- [Environment Variables](#environment-variables)
- [Examples](#examples)

## Overview

The `fabrica` CLI provides commands for initializing projects, adding resources, generating code, and managing API versions. All commands support both interactive and non-interactive modes.

**Installation:**
```bash
# Latest release
go install github.com/openchami/fabrica/cmd/fabrica@latest

# Or download binary
curl -L https://github.com/openchami/fabrica/releases/latest/download/fabrica-$(uname -s)-$(uname -m) -o fabrica
chmod +x fabrica
sudo mv fabrica /usr/local/bin/
```

**Quick Start:**
```bash
fabrica init my-api               # Initialize project
cd my-api
fabrica add resource Device       # Add resource
fabrica generate                  # Generate code
go run ./cmd/server/              # Run server
```

## Global Options

These options work with all commands:

| Option | Description |
|--------|-------------|
| `--help`, `-h` | Show help for command |
| `--version`, `-v` | Show Fabrica version |

## Commands

### fabrica init

Initialize a new Fabrica project with configuration files and directory structure.

**Usage:**
```bash
fabrica init [project-name] [flags]
```

**Arguments:**
- `project-name` - Name of the project (required, becomes directory name)

**Flags:**

#### Basic Options
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--module <path>` | string | `github.com/user/<project>` | Go module path |
| `--description <text>` | string | - | Project description |
| `--interactive`, `-i` | bool | false | Interactive wizard mode |

#### API Versioning
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--group <name>` | string | `example.fabrica.dev` | API group name |
| `--versions <list>` | string | `v1` | Comma-separated version list |
| `--storage-version <ver>` | string | `v1` | Hub/storage version |

#### Feature Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--auth` | bool | false | Enable authentication scaffolding |
| `--storage` | bool | true | Enable storage backend |
| `--metrics` | bool | false | Enable metrics/monitoring |
| `--events` | bool | false | Enable CloudEvents integration |
| `--reconcile` | bool | false | Enable reconciliation framework |

#### Storage Options
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--storage-type <type>` | string | `file` | Storage backend: `file`, `ent` |
| `--db <driver>` | string | `sqlite` | Database driver: `sqlite`, `postgres`, `mysql` |

#### Event Options
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--events-bus <type>` | string | `memory` | Event bus type: `memory`, `nats`, `kafka` |

#### Reconciliation Options
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--reconcile-workers <n>` | int | 3 | Number of reconciler workers |
| `--reconcile-requeue <ms>` | int | 5 | Default requeue delay (minutes) |

#### Validation Options
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--validation-mode <mode>` | string | `strict` | Validation mode: `strict`, `warn`, `disabled` |

**Examples:**

```bash
# Basic initialization
fabrica init my-api

# Custom module path
fabrica init my-api --module github.com/myorg/my-api

# With database storage
fabrica init my-api --storage-type ent --db postgres

# With events and reconciliation
fabrica init my-api --events --reconcile

# Multiple versions from the start
fabrica init my-api --versions v1alpha1,v1beta1,v1 --storage-version v1

# Interactive mode
fabrica init my-api --interactive
```

**Generated Structure:**
```
my-api/
├── .fabrica.yaml       # Project configuration
├── apis.yaml           # API groups and versions
├── go.mod              # Go module
├── README.md           # Project documentation
├── cmd/
│   └── server/
│       └── main.go     # Server entrypoint
├── apis/               # Resource definitions
│   └── example.fabrica.dev/
│       └── v1/         # Version directory
└── internal/
    ├── storage/        # Storage layer
    └── middleware/     # Middleware
```

---

### fabrica add resource

Add a new resource type to your project.

**Usage:**
```bash
fabrica add resource <ResourceName> [flags]
```

**Arguments:**
- `ResourceName` - Name of the resource (PascalCase, e.g., `Device`, `User`)

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--version <ver>` | string | storage version | Add to specific version |
| `--force` | bool | false | Allow adding to stable versions |

**Examples:**

```bash
# Add to default (storage) version
fabrica add resource Device

# Add to specific version
fabrica add resource Device --version v1alpha1

# Add to stable version (requires --force)
fabrica add resource Device --version v1 --force
```

**What It Does:**
1. Creates `apis/<group>/<version>/<resource>_types.go` with stub struct
2. Updates `apis.yaml` to include the resource
3. Ready for you to define `Spec` and `Status` fields

**Generated Stub:**
```go
// apis/example.fabrica.dev/v1/device_types.go
package v1

import "github.com/openchami/fabrica/pkg/fabrica"

type Device struct {
    APIVersion string           `json:"apiVersion"`
    Kind       string           `json:"kind"`
    Metadata   fabrica.Metadata `json:"metadata"`
    Spec       DeviceSpec       `json:"spec"`
    Status     DeviceStatus     `json:"status,omitempty"`
}

type DeviceSpec struct {
    // TODO: Add your spec fields
}

type DeviceStatus struct {
    // TODO: Add your status fields
}
```

---

### fabrica add version

Add a new API version to your project, optionally copying from an existing version.

**Usage:**
```bash
fabrica add version <version> [flags]
```

**Arguments:**
- `version` - Version name (e.g., `v1alpha1`, `v1beta1`, `v2`)

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from <version>` | string | - | Copy types from existing version |
| `--force` | bool | false | Allow adding stable versions without `alpha`/`beta` suffix |

**Examples:**

```bash
# Add new alpha version
fabrica add version v1alpha1

# Add beta by copying alpha
fabrica add version v1beta1 --from v1alpha1

# Promote to stable
fabrica add version v1 --from v1beta1 --force

# Add new major version
fabrica add version v2alpha1
```

**What It Does:**
1. Creates `apis/<group>/<version>/` directory
2. If `--from` specified, copies all `*_types.go` files from source version
3. Updates `apis.yaml` to include new version in the versions list

**Version Naming:**
- `v<N>alpha<M>` - Alpha quality (e.g., `v1alpha1`, `v2alpha3`)
- `v<N>beta<M>` - Beta quality (e.g., `v1beta1`, `v2beta2`)
- `v<N>` - Stable (e.g., `v1`, `v2`) - requires `--force`

---

### fabrica generate

Generate code from resource definitions (handlers, storage, client, OpenAPI).

**Usage:**
```bash
fabrica generate [flags]
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--handlers` | bool | true | Generate HTTP handlers |
| `--storage` | bool | true | Generate storage layer |
| `--client` | bool | true | Generate HTTP client |
| `--openapi` | bool | true | Generate OpenAPI spec |
| `--all` | bool | true | Generate everything (default) |
| `--debug` | bool | false | Show detailed generation steps |
| `--force` | bool | false | Overwrite existing files without prompting |

**Examples:**

```bash
# Generate everything (default)
fabrica generate

# Generate only handlers and storage
fabrica generate --handlers --storage --client=false --openapi=false

# Debug mode
fabrica generate --debug

# Force overwrite
fabrica generate --force
```

**What It Generates:**

```
Generated Files:
├── cmd/server/
│   ├── *_handlers_generated.go     # CRUD handlers (per resource)
│   ├── models_generated.go         # Request/response models
│   ├── routes_generated.go         # Route registration
│   ├── openapi_generated.go        # OpenAPI spec
│   ├── export.go                   # Export command
│   └── import.go                   # Import command
├── internal/
│   ├── storage/
│   │   ├── storage_generated.go    # Storage functions
│   │   └── ent/                    # Ent schemas (if using Ent)
│   │       ├── schema/
│   │       │   ├── resource.go
│   │       │   ├── label.go
│   │       │   └── annotation.go
│   │       ├── ent_adapter.go
│   │       ├── ent_queries_generated.go
│   │       └── ent_transactions_generated.go
│   └── middleware/
│       ├── validation_middleware_generated.go
│       ├── conditional_middleware_generated.go
│       └── versioning_middleware_generated.go
└── pkg/
    └── client/
        └── client_generated.go     # HTTP client library
```

**Generation Process:**

1. **Registration Phase:**
   - AST-parse `apis/<group>/<version>/*_types.go`
   - Generate `apis/<group>/<version>/register_generated.go`
   - Import resource types

2. **Reflection Phase:**
   - Build and run temporary Go program
   - Use reflection to extract type information
   - Build resource metadata

3. **Template Phase:**
   - Apply embedded templates
   - Generate handlers, storage, client, etc.
   - Format with `gofmt`

4. **Ent Phase (if `--storage-type ent`):**
   - Generate Ent schemas automatically
   - No need to run `fabrica ent generate` manually

**Important:**
- Always run `go mod tidy` after generation
- Files ending in `_generated.go` are completely overwritten
- Your resource definitions (`*_types.go`) are never modified
- Run from project root directory
- Versioned APIs require `pkg/apiversion/registry_generated.go` for apiVersion validation

---

### fabrica ent generate

**Deprecated:** Ent code generation is now automatic during `fabrica generate`.

This command is kept for backward compatibility but does nothing. Ent schemas, adapter, queries, and transactions are generated automatically when you run `fabrica generate` with `--storage-type ent`.

**Usage:**
```bash
fabrica ent generate  # No-op, prints deprecation notice
```

**Migration:**
```bash
# Old workflow
fabrica generate
fabrica ent generate  # Deprecated

# New workflow
fabrica generate      # Generates Ent code automatically
```

---

### fabrica version

Display Fabrica version information.

**Usage:**
```bash
fabrica version
```

**Output:**
```
Fabrica version v0.3.1
  commit: abc123def456
  built: 2025-01-14T10:00:00Z
```

---

## Configuration Files

### .fabrica.yaml

Project configuration with feature flags and settings.

**Location:** Project root

**Structure:**
```yaml
project:
  name: my-api
  module: github.com/myorg/my-api
  description: My awesome API
  created: "2025-01-14T10:00:00Z"

features:
  validation:
    enabled: true
    mode: strict              # strict | warn | disabled

  conditional:
    enabled: true
    etag_algorithm: sha256    # sha256 | md5

  versioning:
    enabled: true
    strategy: header          # header | path | query

  events:
    enabled: true
    bus_type: memory          # memory | nats | kafka
    lifecycle_events: true
    condition_events: true

  reconciliation:
    enabled: true
    worker_count: 3
    requeue_delay: 5          # minutes

  auth:
    enabled: false
    provider: custom          # custom | tokensmith

  storage:
    enabled: true
    type: ent                 # file | ent
    db_driver: postgres       # sqlite | postgres | mysql

generation:
  handlers: true
  storage: true
  client: true
  openapi: true
  middleware: true
  reconciliation: true        # if features.reconciliation.enabled
```

**Editing:**
- Safe to edit manually
- Changes take effect on next `fabrica generate`
- Schema validated on load

---

### apis.yaml

API versioning configuration (hub/spoke pattern).

**Location:** Project root

**Structure:**
```yaml
groups:
  - name: example.fabrica.dev
    storageVersion: v1           # Hub version (for storage)
    versions:                    # All exposed versions
      - v1alpha1
      - v1beta1
      - v1
    resources:                   # Resource kinds
      - Device
      - User
      - Product
```

**Workflow:**
1. `fabrica init` creates with single v1 version
2. `fabrica add resource` adds to resources list
3. `fabrica add version` adds to versions list
4. `fabrica generate` uses to discover resources

**Hub Version:**
- Storage format (all data persisted in this version)
- Other versions are "spokes" that convert to/from hub
- Must be a stable version (no `alpha`/`beta`)

---

## Environment Variables

Configure Fabrica behavior via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `FABRICA_CONFIG` | `.fabrica.yaml` | Path to config file |
| `FABRICA_DEBUG` | `false` | Enable debug logging |
| `FABRICA_FORCE` | `false` | Force overwrite without prompting |
| `FABRICA_TEMPLATE_DIR` | embedded | Custom template directory |

**Event Configuration (runtime):**
| Variable | Default | Description |
|----------|---------|-------------|
| `FABRICA_EVENTS_ENABLED` | from config | Enable/disable events |
| `FABRICA_LIFECYCLE_EVENTS_ENABLED` | from config | Enable lifecycle events |
| `FABRICA_CONDITION_EVENTS_ENABLED` | from config | Enable condition events |
| `FABRICA_EVENT_PREFIX` | `io.fabrica` | Event type prefix |
| `FABRICA_EVENT_SOURCE` | project name | Event source identifier |

**Server Configuration (runtime):**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `HOST` | `0.0.0.0` | HTTP server host |
| `DATABASE_URL` | - | Database connection string |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

**Examples:**
```bash
# Custom config location
FABRICA_CONFIG=/path/to/config.yaml fabrica generate

# Debug mode
FABRICA_DEBUG=true fabrica generate

# Custom event prefix
FABRICA_EVENT_PREFIX=com.mycompany ./my-server

# Database URL
DATABASE_URL="postgres://user:pass@localhost/mydb?sslmode=disable" ./my-server
```

---

## Examples

### Complete Workflow

```bash
# 1. Initialize project
fabrica init device-api \
  --module github.com/myorg/device-api \
  --storage-type ent \
  --db postgres \
  --events \
  --reconcile

cd device-api

# 2. Add resources
fabrica add resource Device
fabrica add resource Sensor

# 3. Edit resource definitions
vim apis/example.fabrica.dev/v1/device_types.go
vim apis/example.fabrica.dev/v1/sensor_types.go

# 4. Generate code
fabrica generate

# 5. Tidy dependencies
go mod tidy

# 6. Run server
DATABASE_URL="postgres://localhost/devices" go run ./cmd/server/

# 7. Test API
curl http://localhost:8080/devices
```

### Multi-Version Workflow

```bash
# Start with alpha
fabrica init my-api --versions v1alpha1 --storage-version v1alpha1
cd my-api

fabrica add resource Device
# ... edit and test ...

# Promote to beta
fabrica add version v1beta1 --from v1alpha1
# ... refine schema ...
fabrica generate

# Promote to stable
fabrica add version v1 --from v1beta1 --force

# Update hub version
vim apis.yaml  # Change storageVersion to v1

fabrica generate
```

### Database Workflow

```bash
# SQLite (development)
fabrica init my-api --storage-type ent --db sqlite
cd my-api
fabrica add resource Device
fabrica generate
go run ./cmd/server/ --database-url "file:./data.db?_fk=1"

# PostgreSQL (production)
fabrica init my-api --storage-type ent --db postgres
cd my-api
fabrica add resource Device
fabrica generate
DATABASE_URL="postgres://user:pass@localhost/mydb" go run ./cmd/server/
```

---

## Common Errors

### "Command not found: fabrica"

**Cause:** Fabrica not in PATH

**Fix:**
```bash
# Check installation
which fabrica

# Add to PATH
export PATH="$PATH:$HOME/go/bin"

# Or reinstall
go install github.com/openchami/fabrica/cmd/fabrica@latest
```

### "go run ./cmd/server: multiple files"

**Cause:** Multiple `.go` files in `cmd/server/`

**Fix:**
```bash
# Use trailing slash
go run ./cmd/server/

# Or build first
go build -o server ./cmd/server
./server
```

### "failed to read module path"

**Cause:** Not in a Go module, or `go.mod` missing

**Fix:**
```bash
# Initialize module first
go mod init github.com/myorg/my-api

# Then run fabrica init
fabrica init my-api
```

### "resource not found" during generate

**Cause:** AST parsing couldn't find resource struct

**Fix:**
- Ensure struct name matches resource name exactly
- Check struct is exported (PascalCase)
- Verify file is in correct version directory
- Run with `--debug` to see parsed resources

---

## See Also

- [Getting Started Guide](../guides/getting-started.md)
- [API Versioning Guide](../guides/versioning.md)
- [Storage Guide](../guides/storage.md)
- [Events Guide](../guides/events.md)
- [Reconciliation Guide](../guides/reconciliation.md)

---

**Next Steps:**
- Review [quickstart](../guides/quickstart.md) for hands-on tutorial
- Explore [examples](../../examples/) for real-world patterns
- Check [architecture](architecture.md) for design details
