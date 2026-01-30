<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Examples

Welcome to the Fabrica examples! These examples introduce new users to Fabrica's code generation capabilities through progressively more complex scenarios.

## 🆕 What's New in v0.4

**Hub/Spoke API Versioning** is now available! See [Example 8: API Versioning](08-api-versioning/) to learn how to:
- Define multiple API versions (v1alpha1, v1beta1, v1) for your resources
- Use a single hub (storage) version with multiple spoke (external) versions
- Automatically convert between versions
- Safely evolve your APIs without breaking changes

Generated resources now use a flattened envelope structure (`APIVersion`, `Kind`, `Metadata`, `Spec`, `Status` fields) for better Go autodoc support. The JSON wire format remains identical for backward compatibility. See the [API Versioning Guide](../docs/guides/versioning.md) for migration details.

## Learning Path

Follow these examples in order to build your understanding:

### 1. [Basic CRUD](01-basic-crud/) - Start Here! ⭐
**Time: 10 minutes**

Learn the fundamentals:
- Creating a new project with `fabrica init`
- Adding resources with `fabrica add resource`
- Generating complete CRUD APIs with `fabrica generate`
- Understanding the resource model (Spec/Status pattern)
- Testing operations with cURL
- Working with generated code

**What you'll build:** A device inventory API with full CRUD operations, generated in seconds.

### 2. [Storage and Authentication](02-storage-auth/) - Essential Skills 🔐
**Time: 20 minutes**

Add production features:
- Configuring different storage backends (file, memory, database)
- Integrating JWT authentication with tokensmith middleware
- Protecting endpoints with role-based access
- Implementing custom validation
- Working with metadata (labels, annotations)

**What you'll build:** A secure device inventory with JWT authentication and persistent storage.

### 3. [FRU Service](03-fru-service/) - Production Features 🔐
**Time: 30 minutes**

Master production features:
- SQLite database with Ent ORM
- Generated middleware (validation, conditional requests, versioning)
- Status lifecycle management
- Kubernetes-style conditions
- Working with metadata (labels, annotations)

**What you'll build:** A field replaceable unit tracking system with persistent storage.

### 4. [CloudEvents Integration](05-cloud-events/) - Event Publishing 📡
**Time: 15 minutes**

Master event-driven patterns:
- CloudEvents-compliant event publishing
- Automatic lifecycle event publishing (create, update, delete)
- Condition change events
- Event subscription and monitoring
- Integration with external event systems

**What you'll build:** A sensor monitoring API with comprehensive event publishing and a real-time event subscriber.

### 5. [Rack Reconciliation](04-rack-reconciliation/) - Event-Driven Architecture 🔄
**Time: 45 minutes**

Master declarative patterns:
- Event-driven reconciliation controllers
- Hierarchical resource provisioning
- Kubernetes-style declarative workflows
- Parent-child resource relationships
- Asynchronous operations with status tracking

**What you'll build:** A data center rack inventory system that automatically provisions child resources (chassis, blades, nodes, BMCs) when a Rack is created.

### 6. [Spec Version History](07-spec-versioning/) - Track Spec Changes 🕘
**Time: 15 minutes**

Keep an immutable history of spec changes:
- Opt-in per-resource spec versioning
- Snapshots on create/update/patch of Spec
- `status.version` shows current spec version
- List/get/delete version history via endpoints

**What you'll build:** A small API with a versioned Sensor resource and a script to exercise version history.

### 7. [Hub/Spoke API Versioning](08-api-versioning/) - Multi-Version APIs 🔄
**Time: 20 minutes**

Master Kubebuilder-style API versioning:
- Hub/spoke versioning model (one storage version, multiple external versions)
- Automatic conversion between versions
- Version negotiation middleware
- Safe API evolution without breaking clients

**What you'll build:** A device management API supporting v1alpha1, v1beta1, and v1 with automatic version conversion.

### 8. [Advanced Ent Storage Features](09-ent-advanced/) - Production Storage 🚀
**Time: 30-45 minutes**

Unlock powerful Ent storage capabilities:
- **Query builders** - Type-safe database queries with label filtering
- **Atomic transactions** - Consistent multi-resource operations
- **Export/import** - Backup, migration, and disaster recovery
- Building complex queries with Ent's fluent API
- Real-world patterns for microservice storage

**Prerequisites:** Understand Ent storage basics from Example 3

**What you'll build:** A system management API with advanced querying, transactions, and data portability. Includes:
- `QueryServers()`, `ListServersByLabels()`, `GetServerByUID()` patterns
- Atomic multi-resource operations with `WithTx()`
- CLI commands for export and import
- Practical handler integration patterns
- Quick-start demo script

**Key Features:**
- Query builder code generation
- Transaction wrapper generation
- Export/import CLI framework
- Real-world patterns (filter, pagination, migration)
- Troubleshooting guide and FAQ

### 9. [Export / Import (Ent)](10-export-import/) - Backup & Migration 📦
**Time:** 10-15 minutes

Ship-ready backup workflows using generated server commands:
- Generated `export` / `import` subcommands (no API server needed)
- Offline backups with JSON/YAML output and per-type organization
- Import modes: upsert / replace / skip with transactional safety
- Works with any Ent-backed project; reuse Example 09 demo server

**Prerequisites:** Ent storage basics

**What you'll build:** A repeatable backup/restore flow with generated CLI commands and JSON/YAML artifacts.

### 10. [Node Service Shim](11-node-service/) - Profiles & NodeSets 🧭
**Time:** 30-40 minutes

Introduce profile-aware node composition:
- Node/NodeSet/ProfileBinding resources
- Reconciliation-based NodeSet resolution
- Profile binding materialization (effective profile/boot/config)
- Ent/SQLite storage with generated CLI

**What you'll build:** A node-service shim that composes inventory + boot + metadata intent and demonstrates profile bindings without relying on SMD groups for config intent.

## Quick Reference

### Example Comparison

| Feature | Basic CRUD | Storage & Auth | FRU Service | CloudEvents | Rack Reconciliation | Ent Advanced | Export/Import |
|---------|------------|----------------|-------------|-------------|---------------------|--------------|---------------|
| CRUD Operations | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Code Generation | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| OpenAPI Spec | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Storage Backends | File | File/DB | DB | File | File | Ent (DB) | Ent (DB) |
| Authentication | ❌ | ✅ JWT | ✅ JWT | ❌ | ❌ | ❌ | ❌ |
| Authorization | ❌ | ✅ RBAC | ✅ RBAC | ❌ | ❌ | ❌ | ❌ |
| Validation | Basic | ✅ Custom | ✅ Custom | Basic | ✅ Custom | ✅ Custom | ✅ Custom |
| Reconciliation | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| CloudEvents | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |
| Event Monitoring | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
| Hierarchical Resources | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| State Machines | ❌ | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ |
| Query Builders | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ Type-safe queries | ✅ via storage |
| Transactions | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ Atomic ops | ✅ atomic imports |
| Export/Import | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ server commands | ✅ server commands |
| Label Filtering | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ Production queries | ✅ via storage |

### Running Examples

Each example demonstrates the complete workflow from initialization to running server:

```bash
#
cd examples/03-fru-service
fabrica init . --events --reconcile
fabrica add resource FRU
# Edit apis/example.fabrica.dev/v1/fru_types.go
fabrica generate
go mod tidy  # Update dependencies
# Uncomment lines in cmd/server/main.go
go run ./cmd/server
```

## Prerequisites

- **Go 1.23+** installed
- **Fabrica CLI** installed: `go install github.com/openchami/fabrica/cmd/fabrica@latest`
- Basic knowledge of:
  - REST APIs
  - Go programming
  - Command line usage

## Getting Help

- **Documentation:** See [../docs/](../docs/) for comprehensive guides
- **Issues:** https://github.com/openchami/fabrica/issues
- **Discussions:** Use GitHub Discussions for questions

## Example Structure

Each example README provides:

```
example-name/
├── README.md              # Step-by-step walkthrough
├── What fabrica init creates
├── What fabrica add resource creates
├── How to customize resources
├── What fabrica generate creates
├── How to test the API
└── Troubleshooting tips
```

## Tips for Learning

1. **Start with Example 1** - Even if you're experienced, it establishes the foundation
2. **Read the README first** - Each example's README explains concepts before code
3. **Follow the steps exactly** - The examples are designed to work step-by-step
4. **Experiment** - Modify resources and regenerate to see what changes
5. **Study the generated code** - Understanding what Fabrica generates helps you extend it

## What Fabrica Generates

### `fabrica init myproject`

Creates complete project structure:
- Project directory with Go module
- `cmd/server/main.go` with commented storage/routes (uncomment after generate)
- `apis.yaml` with API group and version configuration
- `apis/<group>/<version>/` directories for resource definitions
- Documentation and examples

### `fabrica add resource Device`

Creates resource definition template:
- `apis/<group>/<version>/device_types.go` with:
  - Device struct using flattened envelope (APIVersion, Kind, Metadata, Spec, Status)
  - DeviceSpec and DeviceStatus structs
  - Validate() method stub
- Updates `apis.yaml` to include Device in resources list

### `fabrica generate`

Generates complete implementation:
- **HTTP Handlers** - Full CRUD operations (Create, Read, Update, Delete, List)
- **Request/Response Models** - Type-safe models for each endpoint
- **Storage Layer** - File-based storage implementation
- **Route Registration** - Chi router configuration
- **OpenAPI Spec** - Complete API documentation
- **Resource Registry** - Auto-discovery of all resources

### What You Write

- **Resource definitions** - Define your Spec and Status fields in `apis/<group>/<version>/`
- **Custom validation** - Implement domain-specific validation logic
- **Business logic** - Add custom handlers beyond CRUD
- **Reconciliation** - Implement controllers for declarative workflows
- **Version conversions** - Implement conversion functions between API versions (hub/spoke)

## Complete Workflow

```bash
# 1. Create project
fabrica init myapi
cd myapi

# 2. Add resources
fabrica add resource Device
fabrica add resource User

# 3. Customize resources (edit apis/<group>/<version>/*_types.go)
vim apis/example.fabrica.dev/v1/device_types.go

# 4. Generate everything
fabrica generate

# 5. Uncomment in cmd/server/main.go:
#    - Storage initialization
#    - Route registration

# 6. Run!
go run ./cmd/server
```

## Key Features

✅ **Code Generation** - Generate complete CRUD APIs from resource definitions
✅ **Type Safety** - Compile-time validation throughout
✅ **Kubernetes-style** - Resources with APIVersion, Kind, Metadata, Spec, Status
✅ **Validation** - Struct tags + custom validation hooks
✅ **Storage Abstraction** - File-based by default, extensible
✅ **OpenAPI** - Auto-generated documentation

## Common Workflows

### Adding a New Resource

```bash
fabrica add resource MyResource
# Edit apis/<group>/<version>/myresource_types.go
fabrica generate
go mod tidy  # Update dependencies
go run ./cmd/server/
```

### Modifying an Existing Resource

```bash
# Edit apis/<group>/<version>/device_types.go
fabrica generate  # Regenerates handlers/storage
go mod tidy  # Update dependencies
go run ./cmd/server/
```

### Switching Storage Backends

```bash
fabrica init myapi --storage=postgres
# Or edit after init
fabrica generate
go mod tidy  # Update dependencies
```

## Generated Code Overview

### Handlers
- Decode/validate requests
- Create resources with proper metadata
- Store using storage abstraction
- Return type-safe responses

### Storage
- File-based JSON storage (default)
- Thread-safe operations
- CRUD methods per resource type
- Easily swap for database storage

### Models
- Request models with embedded Spec
- Response models matching resource types
- Validation tags throughout

### Routes
- Chi router registration
- RESTful URL patterns: `/{resources}` and `/{resources}/{uid}`
- Proper HTTP methods (POST/GET/PUT/DELETE)

## Next Steps

After completing these examples:

1. **Build Your Own API** - Apply what you've learned to your use case
2. **Explore Advanced Topics** - Check out [../docs/](../docs/) for:
   - API versioning
   - Custom storage backends
   - Policy enforcement
   - Conditional updates
   - Event systems
3. **Contribute** - Share your examples or improvements!

## Development Tips

### Working with Local Fabrica

If developing Fabrica itself, add a replace directive to use local templates:

```go
// In your test project's go.mod
replace github.com/openchami/fabrica => /path/to/local/fabrica
```

### Regenerating Code

The generator is idempotent - safe to run multiple times:

```bash
# After modifying resources
fabrica generate  # Regenerates all code
go mod tidy       # Update dependencies
go build ./cmd/server
```

### Debugging Generated Code

Generated files have `_generated.go` suffix:
- `*_handlers_generated.go` - HTTP handlers
- `models_generated.go` - Request/response types
- `routes_generated.go` - Route registration
- `storage_generated.go` - Storage layer
- `openapi_generated.go` - API spec

Don't edit these - modify resources and regenerate instead!

## Questions?

Each example includes:
- ✅ Detailed step-by-step instructions
- ✅ Explanation of generated code
- ✅ cURL commands to test APIs
- ✅ Troubleshooting tips
- ✅ Common issues and solutions

If you get stuck, check the example's README first, then consult the main documentation in [../docs/](../docs/).

Happy building! 🚀
