<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Code Generation

Fabrica uses template-based code generation to create consistent, production-ready code for REST APIs, storage backends, and client libraries. This document explains how the code generation system works and how to use it effectively.

## Overview

Fabrica uses a **two-phase code generation** approach:

### Phase 1: Resource Registration (`fabrica codegen init`)
1. **Discovers resources** - Scans `pkg/resources/` for resource definitions using AST parsing
2. **Generates registration file** - Creates `pkg/resources/register_generated.go` with imports
3. **Registers types** - Sets up resource metadata for the generator

### Phase 2: Code Generation (`fabrica generate`)
1. **Creates temporary program** - Builds a Go program that imports the registration file
2. **Registers resources** - Uses reflection to extract type information
3. **Loads templates** - Reads embedded Go templates from the binary
4. **Generates code** - Applies templates to create handlers, storage, clients, etc.
5. **Formats output** - Runs `go fmt` on all generated files

All generated files have the suffix `_generated.go` and include a header comment warning not to edit them directly.

**Why two phases?** Go doesn't support dynamic type loading at runtime. The registration file bridges the gap by importing and instantiating resource types, which the generator then introspects via reflection.

## Quick Start

```bash
# Two-phase workflow:

# 1. Initialize code generation (after adding/modifying resources)
fabrica codegen init

# 2. Generate everything (recommended)
fabrica generate

# Or generate specific components
fabrica generate --handlers     # Just HTTP handlers
fabrica generate --storage      # Just storage layer
fabrica generate --client       # Just client library
fabrica generate --openapi      # Just OpenAPI spec

# Or use the Makefile for the complete workflow
make dev                        # Clean, init, generate, and build
```

## Architecture

### Generator Components

The code generator consists of several key parts:

**[pkg/codegen/generator.go](../pkg/codegen/generator.go)**
- Core generator logic
- Template loading and execution
- Resource registration
- Output file management

**[pkg/codegen/templates/](../pkg/codegen/templates/)**
- Go templates for each code artifact
- Embedded in binary using `//go:embed`
- Cannot reference parent directories (must be in package dir)

**[cmd/fabrica/generate.go](../cmd/fabrica/generate.go)**
- CLI command implementation
- Resource discovery via AST parsing
- Orchestrates generation for different targets

### Resource Discovery

The generator automatically discovers resources using Go's AST parser:

```go
// Scans pkg/resources/ and looks for:
type MyResource struct {
    resource.Resource  // ← Must embed resource.Resource
    Spec   MyResourceSpec   `json:"spec"`
    Status MyResourceStatus `json:"status"`
}
```

**Discovery process:**
1. Walk `pkg/resources/` directory tree
2. Parse each `.go` file into an AST
3. Find struct types that embed `resource.Resource`
4. Extract resource name and package information

## Templates

### Template Files

All templates are located in [pkg/codegen/templates/](../pkg/codegen/templates/):

| Template | Purpose | Output Location | Used By |
|----------|---------|-----------------|---------|
| `handlers.go.tmpl` | REST API CRUD handlers | `cmd/server/*_handlers_generated.go` | Server |
| `storage.go.tmpl` | File-based storage operations | `internal/storage/storage_generated.go` | Server (file backend) |
| `storage_ent.go.tmpl` | Ent database storage operations | `internal/storage/storage_generated.go` | Server (ent backend) |
| `routes.go.tmpl` | HTTP route registration | `cmd/server/routes_generated.go` | Server |
| `models.go.tmpl` | Request/response types | `cmd/server/models_generated.go` | Server |
| `openapi.go.tmpl` | OpenAPI 3.0 specification | `cmd/server/openapi_generated.go` | Server |
| `client.go.tmpl` | HTTP client library | `pkg/client/client_generated.go` | Client |
| `client-models.go.tmpl` | Client-side types | `pkg/client/models_generated.go` | Client |
| `client-cmd.go.tmpl` | CLI application (Cobra-based) | `cmd/cli/main_generated.go` | CLI |
| `reconciler.go.tmpl` | Resource reconciliation logic | `pkg/reconcile/*_reconciler_generated.go` | Reconcile |
| `reconciler-registration.go.tmpl` | Reconciler registration | `pkg/reconcile/registration_generated.go` | Reconcile |
| `event-handlers.go.tmpl` | Cross-resource event handlers | `pkg/reconcile/event_handlers_generated.go` | Reconcile |

### Ent (Database) Templates

When using Ent storage (`--storage ent`), additional templates are used:

| Template | Purpose | Output Location |
|----------|---------|-----------------|
| `ent/schema/resource.go.tmpl` | Generic resource schema | `internal/storage/ent/schema/resource.go` |
| `ent/schema/label.go.tmpl` | Resource label schema | `internal/storage/ent/schema/label.go` |
| `ent/schema/annotation.go.tmpl` | Resource annotation schema | `internal/storage/ent/schema/annotation.go` |
| `ent_adapter.go.tmpl` | Adapter between Fabrica and Ent | `internal/storage/ent_adapter.go` |
| `generate.go.tmpl` | Ent code generation directive | `internal/storage/generate.go` |

### Middleware Templates

Fabrica generates several middleware templates for common functionality:

| Template | Purpose | Output Location |
|----------|---------|-----------------|
| `validation_middleware.go.tmpl` | Request validation | `internal/middleware/validation_middleware_generated.go` |
| `versioning_middleware.go.tmpl` | API versioning | `internal/middleware/versioning_middleware_generated.go` |
| `conditional_middleware.go.tmpl` | Conditional requests (ETags) | `internal/middleware/conditional_middleware_generated.go` |

For custom authorization, implement your own middleware in `internal/middleware/`.

### Template Variables

Templates have access to resource metadata:

```go
type ResourceMetadata struct {
    Name         string  // "Device"
    PluralName   string  // "devices"
    Package      string  // "github.com/user/project/pkg/resources/device"
    PackageAlias string  // "device"
    TypeName     string  // "*device.Device"
    SpecType     string  // "device.DeviceSpec"
    StatusType   string  // "device.DeviceStatus"
    URLPath      string  // "/devices"
    StorageName  string  // "Device"
}
```

**Template usage example:**
```go
// In handlers.go.tmpl
func Get{{.Name}}(c fuego.ContextWithBody[GetRequest]) ({{.TypeName}}, error) {
    id := c.PathParam("id")
    return storage.Load{{.StorageName}}(c.Context(), id)
}
```

**Generates:**
```go
// In device_handlers_generated.go
func GetDevice(c fuego.ContextWithBody[GetRequest]) (*device.Device, error) {
    id := c.PathParam("id")
    return storage.LoadDevice(c.Context(), id)
}
```

### Template Functions

Templates can use these helper functions:

| Function | Purpose | Example |
|----------|---------|---------|
| `toLower` | Convert to lowercase | `{{toLower .Name}}` → `device` |
| `toUpper` | Convert to uppercase | `{{toUpper .Name}}` → `DEVICE` |
| `title` | Capitalize first letter | `{{title .PluralName}}` → `Devices` |
| `camelCase` | Convert to camelCase | `{{camelCase .Name}}` → `device` |
| `trimPrefix` | Remove prefix | `{{trimPrefix "v1" .Version}}` → `1` |

## Generation Modes

The generator operates in three modes based on the `PackageName`:

### 1. Server Mode (`PackageName: "main"`)

Generates complete server-side code:
- `GenerateHandlers()` - REST API endpoints
- `GenerateStorage()` - Data persistence layer
- `GenerateRoutes()` - URL routing configuration
- `GenerateModels()` - Request/response types
- `GenerateOpenAPI()` - OpenAPI specification
- `GenerateMiddleware()` - Validation, versioning, conditional requests

**Output:** Files in `cmd/server/`, `internal/storage/`, and `internal/middleware/`

### 2. Client Mode (`PackageName: "client"`)

Generates client library code:
- `GenerateClient()` - HTTP client with CRUD methods
- `GenerateClientModels()` - Client-side data types

**Output:** Files in `pkg/client/`

### 3. Reconcile Mode (`PackageName: "reconcile"`)

Generates reconciliation code for eventual consistency:
- `GenerateReconcilers()` - Resource reconciliation logic
- `GenerateReconcilerRegistration()` - Registration code
- `GenerateEventHandlers()` - Cross-resource event handling

**Output:** Files in `pkg/reconcile/`

## Storage Backend Selection

The generator adapts output based on storage type:

### File Storage (Default)

```go
gen := codegen.NewGenerator(outputDir, "main", modulePath)
gen.SetStorageType("file")  // or omit, it's the default
gen.GenerateStorage()
```

**Uses:** `storage.go.tmpl`
**Creates:** JSON file-based persistence in `./data/`

### Ent Storage (Database)

```go
gen := codegen.NewGenerator(outputDir, "main", modulePath)
gen.SetStorageType("ent")
gen.SetDBDriver("postgres")  // or "mysql", "sqlite"
gen.GenerateStorage()
gen.GenerateEntSchemas()
gen.GenerateEntAdapter()
```

**Uses:** `storage_ent.go.tmpl` + Ent templates
**Creates:** Database-backed storage with migrations

## How It Works

### 1. Template Embedding

Templates are embedded in the binary using Go's `embed` directive:

```go
// pkg/codegen/generator.go
//go:embed templates/*
var embeddedTemplates embed.FS
```

**Why this matters:**
- CLI works when installed via `go install` (templates travel with binary)
- No need to distribute template files separately
- Templates must be in or under the package directory (can't use `../../templates`)

### 2. Template Loading

```go
func (g *Generator) LoadTemplates() error {
    // Read from embedded filesystem (not disk!)
    content, err := embeddedTemplates.ReadFile("templates/handlers.go.tmpl")

    // Parse with helper functions
    tmpl, err := template.New("handlers").Funcs(templateFuncs).Parse(string(content))

    g.Templates["handlers"] = tmpl
    return nil
}
```

### 3. Code Generation

```go
func (g *Generator) GenerateHandlers() error {
    for _, resource := range g.Resources {
        var buf bytes.Buffer

        // Execute template with resource metadata
        err := g.Templates["handlers"].Execute(&buf, resource)

        // Format with go fmt
        formatted, err := format.Source(buf.Bytes())

        // Write to file
        filename := fmt.Sprintf("%s_handlers_generated.go", strings.ToLower(resource.Name))
        os.WriteFile(filepath.Join(g.OutputDir, filename), formatted, 0644)
    }
    return nil
}
```

### 4. Resource Registration

Resources are registered using a two-phase approach:

**Phase 1: Generate Registration File**
```bash
fabrica codegen init
```

This scans `pkg/resources/` and creates `pkg/resources/register_generated.go`:
```go
// Code generated by fabrica codegen init. DO NOT EDIT.
package resources

import (
	"fmt"
	"github.com/alexlovelltroy/fabrica/pkg/codegen"
	"github.com/user/project/pkg/resources/device"
)

func RegisterAllResources(gen *codegen.Generator) error {
	if err := gen.RegisterResource(&device.Device{}); err != nil {
		return fmt.Errorf("failed to register Device: %w", err)
	}
	return nil
}
```

**Phase 2: Use Registration for Generation**

When you run `fabrica generate`, it:
1. Creates a temporary Go program
2. Imports the registration file
3. Calls `RegisterAllResources()` to register discovered types via reflection
4. Executes templates with registered resource metadata

## Common Workflows

### Using the Makefile

The generated `Makefile` provides convenient targets for development:

```bash
# Complete development workflow (clean, init, generate, build)
make dev

# Individual targets
make codegen-init   # Initialize code generation
make generate       # Generate handlers, storage, and OpenAPI
make build          # Build the server
make run            # Build and run the server
make test           # Run tests
make clean          # Remove all generated files and binaries
```

**What `make dev` does:**
1. Removes all generated files (`make clean`)
2. Scans and registers resources (`fabrica codegen init`)
3. Generates all code (`fabrica generate --handlers --storage --openapi`)
4. Builds the server binary
5. Reports success

This is the recommended workflow when adding or modifying resources.

### Adding a New Resource

```bash
# 1. Create resource definition
fabrica add resource Product

# 2. Customize the resource
vim pkg/resources/product/product.go

# 3. Initialize code generation (register the new resource)
fabrica codegen init

# 4. Generate code
fabrica generate

# 5. Build and run
go build -o bin/server cmd/server/*.go
./bin/server

# Or use the Makefile for steps 3-5:
make dev
```

### Modifying Generated Code Behavior

**Don't edit generated files directly!** Instead:

```bash
# 1. Find the template
# Generated file says: "Generated from: pkg/codegen/templates/handlers.go.tmpl"

# 2. Edit the template
vim pkg/codegen/templates/handlers.go.tmpl

# 3. Regenerate
fabrica generate

# 4. Verify changes
git diff cmd/server/
```

### Adding a New Endpoint

**Example: Add a count endpoint for each resource**

**Edit `handlers.go.tmpl`:**
```go
// Add this function to the template
func Get{{.Name}}Count(c fuego.ContextNoBody) (map[string]int, error) {
    resources, err := storage.LoadAll{{.StorageName}}s(c.Context())
    if err != nil {
        return nil, err
    }
    return map[string]int{"count": len(resources)}, nil
}
```

**Edit `routes.go.tmpl`:**
```go
// Add this route registration
fuego.Get(server, "{{.URLPath}}/count", Get{{.Name}}Count)
```

**Regenerate:**
```bash
fabrica generate
```

**Result:** Every resource now has a `/resources/count` endpoint!

### Switching Storage Backends

**From file to database:**

```bash
# 1. Regenerate with ent storage
# (Edit your project's generation code to set storage type)

# 2. Create database
createdb myapp

# 3. Generate Ent schemas
fabrica generate

# 4. Run migrations
cd internal/storage && go generate ./ent

# 5. Update main.go to use Ent backend
```

## Generated File Structure

After running `fabrica codegen init` and `fabrica generate` on a project with a `Device` resource:

```
myproject/
├── cmd/server/
│   ├── main.go                           # Server entry point (user-maintained)
│   ├── device_handlers_generated.go      # CRUD handlers for Device
│   ├── routes_generated.go               # Route registration
│   ├── models_generated.go               # Request/response types + helpers
│   └── openapi_generated.go              # OpenAPI spec
├── internal/storage/
│   └── storage_generated.go              # Storage wrappers using fabrica/pkg/storage
├── pkg/client/
│   ├── client_generated.go               # HTTP client
│   └── models_generated.go               # Client types
├── pkg/resources/
│   ├── register_generated.go             # Resource registration (from codegen init)
│   └── device/
│       └── device.go                     # Resource definition (user-maintained)
└── Makefile                              # Build automation with dev workflow
```

## Advanced Features

### Multi-Version Support

Resources can have multiple schema versions:

```go
gen := codegen.NewGenerator("cmd/server", "main", modulePath)
gen.RegisterResource(&device.Device{})

// Add a new version
gen.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v2",
    IsDefault:  false,
    Stability:  "beta",
    Deprecated: false,
    SpecType:   "device.DeviceV2Spec",
})
```

### Custom Middleware

Add custom authentication/authorization middleware:

```go
// In internal/middleware/auth.go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Implement your auth logic
        next.ServeHTTP(w, r)
    })
}
```

Apply middleware in your routes or main.go.

### Custom Template Functions

Extend template capabilities:

```go
// In pkg/codegen/generator.go
var templateFuncs = template.FuncMap{
    "toLower": strings.ToLower,
    "myCustomFunc": func(s string) string {
        // Your logic here
    },
}
```

## Troubleshooting

### Templates Not Found

**Error:** `failed to read embedded template templates/handlers.go.tmpl: no such file or directory`

**Cause:** Templates not embedded properly (usually during development)

**Fix:**
- Ensure templates are in `pkg/codegen/templates/`
- Check `//go:embed templates/*` directive exists
- Rebuild: `go build -o bin/fabrica cmd/fabrica/*.go`

### Resource Not Discovered

**Error:** `No resources found in pkg/resources/`

**Cause:** Resource doesn't embed `resource.Resource` or file doesn't parse

**Fix:**
```go
// Make sure your resource looks like this:
type MyResource struct {
    resource.Resource  // ← Must be present!
    Spec   MyResourceSpec
    Status MyResourceStatus
}
```

### Registration File Not Found

**Error:** `registration file not found: run 'fabrica codegen init' first`

**Cause:** You ran `fabrica generate` without first running `fabrica codegen init`

**Fix:**
```bash
# Run codegen init to create the registration file
fabrica codegen init

# Then generate code
fabrica generate

# Or use the Makefile which does both
make dev
```

### Generated Code Won't Compile

**Error:** `undefined: device.Device`

**Cause:** Missing import or incorrect package path

**Fix:**
- Run `go mod tidy`
- Check that resource package is in correct location
- Verify `go.mod` module path matches template's ModulePath

### Format Errors

**Error:** `failed to format generated code: expected ';', found '}'`

**Cause:** Template produces invalid Go syntax

**Fix:**
- Check template for syntax errors
- Test template with `go run cmd/fabrica/main.go generate`
- Look at the unformatted output (comment out `format.Source()` temporarily)

## Best Practices

### Template Maintenance

1. **Keep templates focused** - Each template should generate one type of artifact
2. **Use descriptive comments** - Help future developers understand generated code
3. **Include error handling** - Generated code should be robust
4. **Follow Go conventions** - Generated code should be idiomatic
5. **Version control templates** - Templates are code, treat them as such

### Resource Design

1. **Always embed `resource.Resource`** - Required for discovery
2. **Use meaningful names** - Resource names become URLs (`/devices`, `/products`)
3. **Validate thoroughly** - Use struct tags: `validate:"required,email"`
4. **Document fields** - Comments in resource become OpenAPI descriptions

### Testing Generated Code

```bash
# 1. Generate code
fabrica generate

# 2. Verify it compiles
go build ./cmd/server

# 3. Run tests
go test ./...

# 4. Check formatting
go fmt ./...

# 5. Lint
golangci-lint run
```

### Customization Strategy

**Instead of editing generated files:**

1. **Extend handlers** - Create separate files with additional endpoints
2. **Wrap storage** - Add caching layer on top of generated storage
3. **Customize routes** - Register additional routes in `main.go`
4. **Override templates** - Modify templates to change default behavior

## Performance Considerations

### Template Parsing

Templates are parsed once during `LoadTemplates()`:
- **Fast:** Templates cached in memory
- **Efficient:** Reused for multiple resources
- **Concurrent-safe:** Read-only after loading

### Code Generation Speed

For a project with 10 resources:
- Template loading: ~50ms
- Code generation: ~200ms
- Go formatting: ~500ms
- **Total: ~750ms**

Generation is fast enough to run on every code change.

### Output Size

Generated code size (per resource):
- Handlers: ~300 lines
- Storage: ~200 lines
- Routes: ~50 lines
- Client: ~150 lines
- **Total: ~700 lines per resource**

This is acceptable for generated code and provides good readability.

## Related Documentation

- [Getting Started Guide](getting-started.md) - Learn Fabrica basics
- [Resource Guide](resources.md) - Define custom resources
- [Storage Guide](storage.md) - Choose storage backends
- [Templates README](../pkg/codegen/templates/README.md) - Template development

## Contributing

To improve code generation:

1. **Identify pain point** - What's repetitive or error-prone?
2. **Modify template** - Edit the relevant `.tmpl` file
3. **Test thoroughly** - Regenerate and verify behavior
4. **Document changes** - Update this guide and template comments
5. **Submit PR** - Share improvements with the community

## Summary

Fabrica's code generation system:
- **Two-phase approach** - Registration file generation, then code generation
- **Discovers resources** automatically via AST parsing
- **Uses reflection** to extract complete type information
- **Loads templates** from embedded filesystem
- **Generates code** for handlers, storage, routes, models, and OpenAPI specs
- **Formats output** with `go fmt`
- **Integrates with fabrica/pkg/storage** for persistence
- **Supports customization** through template editing
- **Makefile automation** with `make dev` workflow

The result: **Write resource definitions once, get complete CRUD APIs automatically.**

### Current Implementation Status

**✅ Working:**
- Resource discovery via AST parsing
- Two-phase registration and generation
- HTTP handlers with CRUD operations
- File-based storage wrappers
- Ent (database) storage backend
- Route registration
- Request/response models with helper functions
- OpenAPI specification generation
- Middleware (validation, versioning, conditional requests)
- Client library generation
- CLI application generation
- Reconciler generation
- Event system integration
- Makefile with dev workflow

**Note:** The core infrastructure is complete and production-ready. For authentication/authorization, implement custom middleware in `internal/middleware/`.
