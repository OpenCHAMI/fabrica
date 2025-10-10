<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica 🏗️

> Build production-ready REST APIs in Go with automatic code generation

[![REUSE status](https://api.reuse.software/badge/github.com/alexlovelltroy/fabrica)](https://api.reuse.software/info/github.com/alexlovelltroy/fabrica)[![golangci-lint](https://github.com/alexlovelltroy/fabrica/actions/workflows/lint.yaml/badge.svg)](https://github.com/alexlovelltroy/fabrica/actions/workflows/lint.yaml)
[![Build](https://github.com/alexlovelltroy/fabrica/actions/workflows/release.yaml/badge.svg)](https://github.com/alexlovelltroy/fabrica/actions/workflows/release.yaml)
[![Release](https://img.shields.io/github/v/release/alexlovelltroy/fabrica?sort=semver)](https://github.com/alexlovelltroy/fabrica/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/alexlovelltroy/fabrica.svg)](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexlovelltroy/fabrica)](https://goreportcard.com/report/github.com/alexlovelltroy/fabrica)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/alexlovelltroy/fabrica/badge)](https://securityscorecards.dev/viewer/?uri=github.com/alexlovelltroy/fabrica)

**Define once. Generate everything.**

```bash
$ fabrica init myapp
$ fabrica add resource Device
$ fabrica generate
✅ Complete REST API ready
```

## What You Get

- ✅ **REST API** - Full CRUD handlers with validation
- ✅ **Storage** - File-based or database (Ent) backends
- ✅ **Client Library** - Type-safe Go client
- ✅ **OpenAPI Spec** - Auto-generated documentation
- ✅ **Authorization** - RBAC/ABAC with Casbin integration
- ✅ **Kubernetes-style** - Familiar resource patterns

## Quick Start

### Install

```bash
go install github.com/alexlovelltroy/fabrica/cmd/fabrica@latest
```

### Create Your First API

```bash
# 1. Initialize project
fabrica init myapp
cd myapp

# 2. Add a resource
fabrica add resource Product

# 3. Customize (edit pkg/resources/product/product.go)
# Add fields to ProductSpec:
#   Price  float64 `json:"price" validate:"required,gt=0"`
#   Stock  int     `json:"stock" validate:"min=0"`

# 4. Generate code
go mod tidy
fabrica generate

# 5. Run
go run cmd/server/main.go
```

Your API is now running at `http://localhost:8080`!

## Example

**Define your resource:**

```go
type Product struct {
    resource.Resource
    Spec   ProductSpec   `json:"spec"`
    Status ProductStatus `json:"status,omitempty"`
}

type ProductSpec struct {
    Name  string  `json:"name" validate:"required"`
    Price float64 `json:"price" validate:"required,gt=0"`
    Stock int     `json:"stock" validate:"min=0"`
}
```

**Generated endpoints:**

```bash
POST   /products       # Create
GET    /products       # List all
GET    /products/{id}  # Get one
PUT    /products/{id}  # Update
DELETE /products/{id}  # Delete
```

**Use the API:**

```bash
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "laptop",
    "price": 1999.99,
    "stock": 42
  }'
```

## Key Features

| Feature | Description |
|---------|-------------|
| **Code Generation** | Generate handlers, storage, clients from resource definitions |
| **Validation** | Struct tags + Kubernetes validators + custom logic |
| **Storage Backends** | File-based (development) or Ent/database (production) |
| **Authorization** | Built-in RBAC/ABAC with Casbin integration |
| **Multi-Version APIs** | Support multiple schema versions simultaneously |
| **Type Safety** | Full type safety across server, storage, and client |
| **Events & Reconciliation** | CloudEvents + Kubernetes-style controllers |

## Documentation

- **[Getting Started](docs/getting-started.md)** - Detailed tutorial
- **[Quick Start Guide](docs/quickstart.md)** - 30-minute walkthrough
- **[Resource Model](docs/resource-model.md)** - Understanding resources
- **[Code Generation](docs/codegen.md)** - How generation works
- **[Authorization](docs/policy-casbin.md)** - RBAC/ABAC setup
- **[Storage Backends](docs/storage.md)** - File vs database
- **[API Reference](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)** - Full API docs

## Architecture

```
┌─────────────────┐
│  Your Resource  │  (Define once)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ fabrica generate│  (Run once)
└────────┬────────┘
         │
         ├─► REST Handlers  (CRUD operations)
         ├─► Storage Layer  (File or DB)
         ├─► Client Library (Type-safe)
         └─► OpenAPI Spec   (Documentation)
```

## Project Modes

Choose your complexity level:

- **Simple** - Just a REST API, no Kubernetes concepts
- **Standard** - Full resource model (recommended)
- **Expert** - Minimal scaffolding, maximum control

```bash
fabrica init myapp --mode=simple   # Easy mode
fabrica init myapp --mode=standard # Full power (default)
```

## Examples

See [examples/](examples/) directory for complete working examples:

- Basic CRUD API
- Multi-version resources
- Custom validation
- Authorization policies
- Event-driven workflows

## Requirements

- Go 1.23 or later
- That's it!

## Status

**Version:** v0.2.5
**Status:** Alpha Quality

✅ Core features stable and tested
✅ Used in production at OpenCHAMI

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md)

```bash
# Run tests
go test ./...

# Run linter
golangci-lint run
```

## License

MIT License - See [MIT.txt](LICENSES/MIT.txt)

## Links

- [GitHub](https://github.com/alexlovelltroy/fabrica)
- [Documentation](docs/)
- [Issues](https://github.com/alexlovelltroy/fabrica/issues)
- [Releases](https://github.com/alexlovelltroy/fabrica/releases)

---

**Built with ❤️ for developers who want to focus on business logic, not boilerplate.**
