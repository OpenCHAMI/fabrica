<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Examples

Welcome to the Fabrica examples! These examples introduce new users to Fabrica's code generation capabilities through progressively more complex scenarios.

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

### 4. [Rack Reconciliation](04-rack-reconciliation/) - Event-Driven Architecture 🔄
**Time: 45 minutes**

Master declarative patterns:
- Event-driven reconciliation controllers
- Hierarchical resource provisioning
- Kubernetes-style declarative workflows
- Parent-child resource relationships
- Asynchronous operations with status tracking

**What you'll build:** A data center rack inventory system that automatically provisions child resources (chassis, blades, nodes, BMCs) when a Rack is created.

## Quick Reference

### Example Comparison

| Feature | Basic CRUD | Storage & Auth | FRU Service | Rack Reconciliation |
|---------|------------|----------------|-------------|---------------------|
| CRUD Operations | ✅ | ✅ | ✅ | ✅ |
| Code Generation | ✅ | ✅ | ✅ | ✅ |
| OpenAPI Spec | ✅ | ✅ | ✅ | ✅ |
| Storage Backends | File | File/DB | DB | File |
| Authentication | ❌ | ✅ JWT | ✅ JWT | ❌ |
| Authorization | ❌ | ✅ RBAC | ✅ RBAC | ❌ |
| Validation | Basic | ✅ Custom | ✅ Custom | ✅ Custom |
| Reconciliation | ❌ | ❌ | ❌ | ✅ |
| Event-Driven | ❌ | ❌ | ❌ | ✅ |
| Hierarchical Resources | ❌ | ❌ | ❌ | ✅ |
| State Machines | ❌ | ❌ | ✅ | ✅ |
| Events | ❌ | ❌ | ❌ | ✅ |

### Running Examples

Each example demonstrates the complete workflow from initialization to running server:

```bash
#
cd examples/03-fru-service
fabrica init . --events --reconcile
fabrica add resource FRU
# Edit pkg/resources/fru/fru.go
fabrica generate
# Uncomment lines in cmd/server/main.go
go run cmd/server/main.go
```

## Prerequisites

- **Go 1.23+** installed
- **Fabrica CLI** installed: `go install github.com/alexlovelltroy/fabrica/cmd/fabrica@latest`
- Basic knowledge of:
  - REST APIs
  - Go programming
  - Command line usage

## Getting Help

- **Documentation:** See [../docs/](../docs/) for comprehensive guides
- **Issues:** https://github.com/alexlovelltroy/fabrica/issues
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
- Empty `pkg/resources/` directory
- Documentation and examples

### `fabrica add resource Device`

Creates resource definition template:
- `pkg/resources/device/device.go` with:
  - Device struct embedding `resource.Resource`
  - DeviceSpec and DeviceStatus structs
  - Validate() method stub
  - Resource registration

### `fabrica generate`

Generates complete implementation:
- **HTTP Handlers** - Full CRUD operations (Create, Read, Update, Delete, List)
- **Request/Response Models** - Type-safe models for each endpoint
- **Storage Layer** - File-based storage implementation
- **Route Registration** - Chi router configuration
- **OpenAPI Spec** - Complete API documentation
- **Resource Registry** - Auto-discovery of all resources

### What You Write

- **Resource definitions** - Define your Spec and Status fields
- **Custom validation** - Implement domain-specific validation logic
- **Business logic** - Add custom handlers beyond CRUD
- **Reconciliation** - Implement controllers for declarative workflows

## Complete Workflow

```bash
# 1. Create project
fabrica init myapi
cd myapi

# 2. Add resources
fabrica add resource Device
fabrica add resource User

# 3. Customize resources (edit pkg/resources/*/...)
vim pkg/resources/device/device.go

# 4. Generate everything
fabrica generate

# 5. Uncomment in cmd/server/main.go:
#    - Storage initialization
#    - Route registration

# 6. Run!
go run cmd/server/main.go
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
# Edit pkg/resources/myresource/myresource.go
fabrica generate
go run cmd/server/main.go
```

### Modifying an Existing Resource

```bash
# Edit pkg/resources/device/device.go
fabrica generate  # Regenerates handlers/storage
go run cmd/server/main.go
```

### Switching Storage Backends

```bash
fabrica init myapi --storage=postgres
# Or edit after init
fabrica generate
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
replace github.com/alexlovelltroy/fabrica => /path/to/local/fabrica
```

### Regenerating Code

The generator is idempotent - safe to run multiple times:

```bash
# After modifying resources
fabrica generate  # Regenerates all code
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
