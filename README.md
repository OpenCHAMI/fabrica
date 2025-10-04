<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica 🏗️

> A powerful Go framework for building resource-based REST APIs with automatic code generation, multi-version schema support, and pluggable storage backends.

## 🎯 Overview

Fabrica provides everything you need to build production-ready REST APIs with minimal boilerplate:

- **🚀 Automatic Code Generation** - Define resources, generate REST API, storage, and client code
- **📚 Multi-Version Schema Support** - Run multiple API versions simultaneously with automatic conversion
- **🔌 Pluggable Storage** - File-based, database, or custom storage backends
- **🔒 Flexible Authorization** - Built-in policy framework for RBAC, ABAC, and custom policies
- **⚡ Type-Safe** - Full type safety across server, storage, and client
- **📖 Kubernetes-Style Resources** - Familiar APIVersion/Kind/Metadata/Spec/Status pattern
- **📡 Event System** - CloudEvents-compliant event bus with wildcard subscriptions
- **🔄 Reconciliation Framework** - Kubernetes-style controllers for declarative resource management
- **✅ Comprehensive Validation** - Struct tags + K8s validators + custom business logic (NEW!)
- **🏷️ Conditional Requests** - ETags, If-Match, optimistic concurrency control (NEW!)
- **🔧 PATCH Operations** - JSON Merge Patch, JSON Patch, and shorthand patches (NEW!)

## ✨ Quick Start

### 30-Second Example

**1. Define your resource:**

```go
// pkg/resources/device/device.go
package device

import "github.com/alexlovelltroy/fabrica/pkg/resource"

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Model    string `json:"model"`
}

type DeviceStatus struct {
    Active     bool   `json:"active"`
    LastSeen   string `json:"lastSeen,omitempty"`
    IPAddress  string `json:"ipAddress,omitempty"`
}

func init() {
    resource.RegisterResourcePrefix("Device", "dev")
}
```

**2. Generate code:**

```go
// cmd/codegen/main.go
package main

import (
    "github.com/alexlovelltroy/fabrica/pkg/codegen"
    "github.com/yourapp/pkg/resources/device"
)

func main() {
    gen := codegen.NewGenerator("cmd/server", "main", "github.com/yourapp")
    gen.RegisterResource(&device.Device{})
    gen.GenerateAll()
}
```

```bash
go run cmd/codegen/main.go
```

**3. Run your API server:**

```go
// cmd/server/main.go
package main

import (
    "github.com/alexlovelltroy/fabrica/pkg/storage"
    "net/http"
)

func main() {
    backend := storage.NewFileBackend("./data")
    RegisterRoutes(backend)
    http.ListenAndServe(":8080", nil)
}
```

**4. Use your API:**

```bash
# Create a device
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Device",
    "metadata": {"name": "sensor-001"},
    "spec": {
      "name": "Temperature Sensor",
      "location": "Building A",
      "model": "TMP-100"
    }
  }'

# List devices
curl http://localhost:8080/devices

# Get specific device
curl http://localhost:8080/devices/dev-abc123
```

That's it! ✅ You now have a fully functional REST API with CRUD operations, type-safe storage, and automatic UID generation.

## 🔥 Key Features

### Automatic Code Generation

Define your resource once, get everything generated:

```
Your Resource Definition
    ↓
Code Generator
    ↓
Generated Code
    ├─ REST API Handlers (CRUD)
    ├─ Storage Operations
    ├─ HTTP Client Library
    ├─ CLI Commands (optional)
    └─ OpenAPI Specification
```

**Benefits:**
- ✅ Consistency across all resources
- ✅ Type-safe operations everywhere
- ✅ Reduce boilerplate by 90%
- ✅ Focus on business logic, not plumbing

**→ See [Code Generation Guide](docs/codegen.md)**

### Multi-Version Schema Support

Support multiple API versions simultaneously with automatic conversion:

```go
// Register multiple versions
gen.RegisterResource(&device.DeviceV1{})
gen.AddResourceVersion("Device", codegen.SchemaVersion{
    Version: "v2beta1",
    Stability: "beta",
})

// Client requests v1, server has v2
GET /devices/dev-123
Accept: application/json;version=v1

// Automatic conversion happens transparently
```

**Use Cases:**
- 🔄 Maintain backward compatibility
- 🚀 Release new features gradually
- 🛡️ Deprecate old versions gracefully
- 🔀 Migrate clients at their own pace

**→ See [Versioning Guide](docs/versioning.md)**

### Kubernetes-Style Resources

Familiar resource structure for anyone who knows Kubernetes:

```go
type Device struct {
    APIVersion string `json:"apiVersion"` // "v1"
    Kind       string `json:"kind"`       // "Device"
    Metadata   Metadata `json:"metadata"` // Name, UID, labels, annotations
    Spec       DeviceSpec `json:"spec"`   // Desired state
    Status     DeviceStatus `json:"status"` // Observed state
}
```

**Features:**
- 📛 Human-readable names + structured UIDs
- 🏷️ Labels for selection and grouping
- 📝 Annotations for arbitrary metadata
- ⏰ Automatic timestamps (created/updated)
- 🔍 Query by labels or annotations

**→ See [Resource Model Guide](docs/resource-model.md)**

### Pluggable Storage

Switch storage backends without changing your code:

```go
// File-based storage (default)
backend := storage.NewFileBackend("./data")

// Database storage (coming soon)
backend := storage.NewPostgresBackend(connectionString)

// Custom storage
type MyStorage struct{}
func (s *MyStorage) Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error) {
    // Your implementation
}
```

**Storage Backends:**
- 📁 **File Storage** - Zero dependencies, production-ready (default)
- 🗄️ **Ent Storage** - Database-backed with PostgreSQL/MySQL/SQLite (NEW!)
  - Type-safe queries with Ent ORM
  - Automatic migrations
  - Transaction support
  - Advanced filtering and aggregations

```bash
# Initialize with Ent storage
fabrica init my-api --storage=ent --db=postgres
```

**→ See [Storage Guide](docs/storage.md) | [Ent Storage Guide](docs/storage-ent.md)**

### Flexible Authorization

Build custom authorization policies for your resources:

```go
type DevicePolicy struct{}

func (p *DevicePolicy) CanCreate(ctx context.Context, auth *policy.AuthContext, req *http.Request, resource interface{}) policy.PolicyDecision {
    // Only admins can create devices
    if policy.HasRole(auth, "admin") {
        return policy.Allow()
    }
    return policy.Deny("must be admin to create devices")
}

func (p *DevicePolicy) CanGet(ctx context.Context, auth *policy.AuthContext, req *http.Request, resourceUID string) policy.PolicyDecision {
    // Users can view devices in their organization
    org, _ := policy.GetStringClaim(auth, "organization")
    if device.Metadata.Labels["organization"] == org {
        return policy.Allow()
    }
    return policy.Deny("can only view devices in your organization")
}
```

**Patterns:**
- 🔐 RBAC (Role-Based Access Control)
- 📊 ABAC (Attribute-Based Access Control)
- 🎫 JWT claim-based authorization
- 🏢 Multi-tenancy support
- 🎭 Custom authorization logic

**→ See [Policy Guide](docs/policy.md)**

## 📦 Installation

```bash
go get github.com/alexlovelltroy/fabrica
```

**Requirements:**
- Go 1.23 or later

## 🏗️ Architecture

Fabrica follows a clean, layered architecture:

```
┌─────────────────────────────────────────────┐
│         HTTP REST API Layer                 │
│  (Generated handlers with CRUD operations)  │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│       Resource Management Layer             │
│   (Versioning, validation, conversion)      │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│         Storage Backend Layer               │
│  (File, database, or custom persistence)    │
└─────────────────────────────────────────────┘
```

**Key Components:**

- **`pkg/resource/`** - Resource model and UID generation
- **`pkg/codegen/`** - Code generation engine
- **`pkg/storage/`** - Storage interfaces and backends
- **`pkg/policy/`** - Authorization framework
- **`pkg/versioning/`** - Multi-version support
- **`pkg/events/`** - CloudEvents-compliant event system
- **`pkg/reconcile/`** - Reconciliation framework and controllers
- **`templates/`** - Code generation templates

**→ See [Architecture Guide](docs/architecture.md)**

## 🔍 How Does Fabrica Compare?

Wondering how Fabrica stacks up against other Go frameworks like **Go-Fuego**, **Huma**, or **Goa**?

**→ See [Framework Comparison](docs/comparison.md)** for detailed analysis, feature matrices, and guidance on choosing the right framework for your project.

**TL;DR**: Fabrica is the only framework specifically designed for inventory and asset management with built-in storage, events, reconciliation, and multi-version support. For general REST APIs, consider Go-Fuego (simple) or Huma (schema-first). For microservices with gRPC, consider Goa.

## 📚 Documentation

### Getting Started

**New to Fabrica? Start here:**
- **[Quick Start](docs/quickstart.md)** ⚡ - Simple REST API in 30 minutes (no Kubernetes concepts)
- **[Getting Started Guide](docs/getting-started.md)** ⭐ - Full resource model in 2-4 hours
- **[Architecture Overview](docs/architecture.md)** - Design and concepts
- **[Examples](docs/examples.md)** - Real-world use cases

**Choose your learning path:**
- **Beginner** → Start with [Quick Start](docs/quickstart.md) for simple CRUD APIs
- **Intermediate** → Continue with [Getting Started](docs/getting-started.md) for resource management
- **Advanced** → Explore [Reconciliation](docs/reconciliation.md) and [Events](docs/events.md)

### Core Concepts
- **[Resource Model](docs/resource-model.md)** - Understanding resources
- **[Storage System](docs/storage.md)** - Storage backends and patterns
- **[Code Generation](docs/codegen.md)** - Template system guide
- **[Versioning](docs/versioning.md)** - Multi-version support
- **[Authorization](docs/policy.md)** - Policy framework
- **[Events](docs/events.md)** - Event system with CloudEvents
- **[Reconciliation](docs/reconciliation.md)** - Declarative resource management

### Reference
- **[API Reference](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)** - Go package docs
- **[Template Reference](templates/README.md)** - Available templates

### Complete Documentation
- **[Documentation Index](docs/README.md)** - Complete documentation map

## 🚀 Use Cases

### IoT Device Management
```go
type Device struct {
    resource.Resource
    Spec DeviceSpec `json:"spec"`
}
// Generates: /devices API with status tracking
```

### Product Catalog
```go
type Product struct {
    resource.Resource
    Spec ProductSpec `json:"spec"`
}
// Generates: /products API with inventory management
```

### User Management
```go
type User struct {
    resource.Resource
    Spec UserSpec `json:"spec"`
}
// Generates: /users API with RBAC policies
```

### Content Management
```go
type Article struct {
    resource.Resource
    Spec ArticleSpec `json:"spec"`
}
// Generates: /articles API with versioning
```

**→ See [Examples Guide](docs/examples.md) for complete implementations**

## 🎓 Learn More

### Tutorials
1. [Your First Resource](docs/getting-started.md#your-first-resource) - 5 minutes
2. [Add Authorization](docs/policy.md#quick-start) - 10 minutes
3. [Multi-Version API](docs/versioning.md#adding-versions) - 15 minutes
4. [Custom Storage](docs/storage.md#custom-backends) - 20 minutes

### Concepts
- [Why Fabrica?](docs/architecture.md#why-fabrica) - Philosophy and goals
- [Design Principles](docs/architecture.md#design-principles) - Framework design
- [Best Practices](docs/architecture.md#best-practices) - Production patterns

## 🤝 Contributing

We welcome contributions! Here's how to get started:

1. **Read the docs**: [Contributing Guide](CONTRIBUTING.md)
2. **Find an issue**: Check [GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues)
3. **Submit a PR**: Follow the [PR template](CONTRIBUTING.md#pull-requests)

**Quick Contribution Ideas:**
- 📖 Improve documentation
- 🐛 Fix bugs
- ✨ Add features
- 🎨 Add examples
- 🧪 Add tests

## 🔗 Links

- **[GitHub Repository](https://github.com/alexlovelltroy/fabrica)**
- **[Go Package Docs](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)**
- **[Issue Tracker](https://github.com/alexlovelltroy/fabrica/issues)**
- **[Discussions](https://github.com/alexlovelltroy/fabrica/discussions)**

## 📝 License

MIT License - See [LICENSE](LICENSE)

## ⭐ Status

- **Version**: v0.1.0 (Early Development)
- **Go Version**: 1.23+
- **Status**: Alpha - API may change

**Production Readiness:**
- ✅ Core resource system - Stable
- ✅ File storage backend - Stable
- ✅ Code generation - Stable
- ⚠️ Versioning system - Beta
- ⚠️ Policy framework - Beta
- 🚧 Database backends - Coming soon

## 🙏 Acknowledgments

Fabrica is inspired by:
- **Kubernetes** - Resource model and API conventions
- **OpenAPI** - REST API patterns
- **Go** - Simplicity and pragmatism

Built with ❤️ for developers who want to focus on business logic, not boilerplate.

---

**Get Started**: [Getting Started Guide](docs/getting-started.md) | **Questions?** [Open an Issue](https://github.com/alexlovelltroy/fabrica/issues)
