# Framework Comparison: Fabrica vs Go-Fuego vs Huma vs Goa

This document compares Fabrica with three popular Go API frameworks to help you choose the right tool for your project.

## 📊 Quick Comparison Matrix

| Feature | Fabrica | Go-Fuego | Huma | Goa |
|---------|---------|----------|------|-----|
| **Approach** | Resource-Centric | Handler-Centric | Schema-First | Design-First DSL |
| **OpenAPI Generation** | ✅ Template-based | ✅ From code | ✅ From code | ✅ From DSL |
| **Code Generation** | ✅ Full stack | ❌ OpenAPI only | ❌ None | ✅ Full stack |
| **Multi-Version Support** | ✅ Built-in | ❌ Manual | ❌ Manual | ⚠️ Via DSL |
| **Storage Abstraction** | ✅ Pluggable | ❌ None | ❌ None | ❌ None |
| **Event System** | ✅ CloudEvents | ❌ None | ❌ None | ❌ None |
| **Reconciliation** | ✅ K8s-style | ❌ None | ❌ None | ❌ None |
| **Authorization** | ✅ Policy-based | ⚠️ Manual | ⚠️ Manual | ⚠️ Manual |
| **CLI Generation** | ✅ Yes | ❌ No | ❌ No | ✅ Yes |
| **Client Generation** | ✅ Yes | ❌ No | ✅ Via OpenAPI | ✅ Yes |
| **Router** | chi-based | net/http (Go 1.22+) | Router-agnostic | Generated |
| **Learning Curve** | Medium | Low | Low | High |
| **Best For** | Inventory systems | REST APIs | REST APIs | Microservices |
| **Production Ready** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |

---

## 🎯 Detailed Comparison

### Fabrica

**Philosophy**: Resource-centric inventory management framework

**Strengths**:
- 🏗️ **Complete Framework**: Not just an API framework - includes storage, events, reconciliation, versioning
- 📦 **Resource Model**: Kubernetes-style resources with metadata, labels, annotations
- 🔄 **Multi-Version Schema**: Built-in support for v1, v2beta1, etc. with automatic conversion
- 🎨 **Template-Based Generation**: Generate handlers, storage, CLI, clients from resource definitions
- 📊 **Event System**: CloudEvents-compliant event bus for reactive architectures
- ♻️ **Reconciliation**: Kubernetes-style reconciliation loops for declarative management
- 🔐 **Policy Framework**: Pluggable authorization with RBAC/ABAC patterns
- 💾 **Storage Abstraction**: File, database, cloud - swap backends without code changes

**Ideal For**:
- Inventory management systems (IT assets, devices, sensors, products)
- Systems needing multi-version APIs
- Event-driven architectures
- Declarative infrastructure management
- Resource-based CRUD applications

**When to Choose Fabrica**:
- You're building an inventory or asset management system
- You need multi-version API support out of the box
- You want Kubernetes-style resource management
- You need events and reconciliation
- You prefer template-based code generation

**Example Use Cases**:
- HPC hardware inventory (OpenCHAMI)
- IoT device management
- Product catalog systems
- Configuration management databases (CMDB)
- Asset tracking systems

---

### Go-Fuego

**Philosophy**: Modern Go API framework with automatic OpenAPI generation from code

**Strengths**:
- 🚀 **Modern Go**: Built on Go 1.22+ net/http with generics
- 📝 **OpenAPI from Code**: Automatic OpenAPI 3 generation without comments or YAML
- 🔌 **Zero Lock-in**: 100% net/http compatible, use any middleware
- ⚡ **Low Boilerplate**: Minimal code for handlers with automatic serialization
- 🎯 **Simple API**: Clean, intuitive API inspired by Nest.js
- 🔄 **Adaptors**: Plugin to existing Gin/Echo apps
- ✅ **Built-in Validation**: go-playground/validator integration

**Ideal For**:
- Modern REST APIs with OpenAPI documentation
- Teams migrating from Gin/Echo wanting OpenAPI
- Projects requiring net/http compatibility
- Developers who prefer code-first approaches

**When to Choose Go-Fuego**:
- You want automatic OpenAPI without YAML files
- You value net/http compatibility
- You need a simple, modern API framework
- You're starting a new REST API project
- You don't need storage, events, or reconciliation

**Example Use Cases**:
- Microservices REST APIs
- API gateways
- Backend services for web/mobile apps
- Public APIs with OpenAPI docs

---

### Huma

**Philosophy**: Schema-first REST/RPC framework with comprehensive OpenAPI 3.1 support

**Strengths**:
- 📋 **OpenAPI 3.1**: Full OpenAPI 3.1 and JSON Schema support
- 🔀 **Router Agnostic**: Works with chi, gin, fiber, gorilla/mux, stdlib
- 📦 **Content Types**: JSON, CBOR, with gzip/Brotli encoding
- ✅ **Type Safety**: Static typing for all inputs/outputs
- 🔧 **Conditional Requests**: If-Match, If-Unmodified-Since support
- 🩹 **Auto PATCH**: Automatic JSON Patch/Merge Patch generation
- 🎨 **Beautiful Docs**: Stoplight Elements integration
- ⚡ **Production Proven**: Used by large companies with millions of users

**Ideal For**:
- REST APIs requiring comprehensive OpenAPI 3.1
- Projects needing multiple content type support (JSON, CBOR)
- Teams wanting router flexibility
- APIs with complex validation requirements

**When to Choose Huma**:
- You need OpenAPI 3.1 (vs 3.0)
- You want router flexibility
- You need conditional request support
- You value comprehensive validation
- You want automatic PATCH generation

**Example Use Cases**:
- Enterprise REST APIs
- APIs with complex schemas
- Multi-tenant SaaS platforms
- APIs requiring content negotiation

---

### Goa

**Philosophy**: Design-first microservices framework with DSL-driven code generation

**Strengths**:
- 🎨 **Design-First DSL**: Express APIs in elegant, type-safe DSL
- ⚙️ **Full Stack Generation**: 30-50% of codebase auto-generated
- 🔀 **Multi-Transport**: HTTP, gRPC, JSON-RPC 2.0 (WebSocket/SSE)
- 🤖 **AI-Powered**: AI design wizard for natural language API creation
- 📚 **Zero Drift**: Design, code, and docs always in sync
- 🛡️ **Enterprise Features**: Built-in validation, error handling, middleware
- 📦 **Complete Tooling**: Server, client, CLI, OpenAPI, Protocol Buffers

**Ideal For**:
- Microservices architectures
- Teams valuing design-first development
- Projects needing multiple transports (HTTP + gRPC)
- Organizations with strict API governance
- Enterprise applications

**When to Choose Goa**:
- You prefer design-first over code-first
- You need gRPC and HTTP from same design
- You want comprehensive code generation
- You value design-implementation consistency
- You need enterprise-grade governance

**Example Use Cases**:
- Microservices platforms
- Multi-protocol APIs (REST + gRPC)
- Enterprise service architectures
- APIs with complex business logic
- Services requiring strict contracts

---

## 🔍 Head-to-Head Comparisons

### Fabrica vs Go-Fuego

**Similarities**:
- Both generate OpenAPI documentation
- Both support modern Go patterns
- Both have clean, intuitive APIs

**Key Differences**:
- **Scope**: Fabrica is a complete framework (storage, events, reconciliation), Fuego is API-focused
- **Resources**: Fabrica is resource-centric, Fuego is handler-centric
- **Generation**: Fabrica generates full stack (handlers, storage, CLI, clients), Fuego generates OpenAPI only
- **Versioning**: Fabrica has built-in multi-version support, Fuego requires manual versioning
- **Events**: Fabrica includes CloudEvents bus, Fuego has none

**Choose Fabrica if**: You need a complete inventory framework with storage and events
**Choose Fuego if**: You just need a modern REST API with OpenAPI docs

---

### Fabrica vs Huma

**Similarities**:
- Both generate OpenAPI from code
- Both support multiple routers (Fabrica: chi, Huma: router-agnostic)
- Both emphasize type safety
- Both are production-ready

**Key Differences**:
- **Philosophy**: Fabrica is resource-centric, Huma is schema-first
- **Scope**: Fabrica includes storage/events/reconciliation, Huma is API-focused
- **Versioning**: Fabrica has built-in multi-version support, Huma requires manual versioning
- **Code Gen**: Fabrica generates full stack, Huma generates OpenAPI only
- **Resources**: Fabrica has Kubernetes-style resources, Huma uses standard Go structs

**Choose Fabrica if**: You're building an inventory system with resources
**Choose Huma if**: You need a flexible schema-first REST API framework

---

### Fabrica vs Goa

**Similarities**:
- Both generate comprehensive code (handlers, clients, CLI)
- Both emphasize design-code consistency
- Both support OpenAPI generation
- Both target enterprise use cases

**Key Differences**:
- **Approach**: Fabrica is resource-centric, Goa is DSL-first
- **Design Language**: Fabrica uses Go structs + templates, Goa uses custom DSL
- **Domain**: Fabrica is inventory-focused, Goa is general microservices
- **Built-ins**: Fabrica includes storage/events/reconciliation, Goa is API/transport focused
- **Learning Curve**: Fabrica uses familiar Go patterns, Goa requires learning DSL
- **Transports**: Fabrica is HTTP-focused, Goa supports HTTP/gRPC/JSON-RPC

**Choose Fabrica if**: You're building inventory systems with resources
**Choose Goa if**: You need multi-protocol microservices with design governance

---

## 🎨 Code Comparison

### Defining a Resource/Endpoint

**Fabrica**:
```go
type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status"`
}

type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
}

// Register and generate everything
gen := codegen.NewGenerator("./gen", "main", "myapp")
gen.RegisterResource(&Device{})
gen.GenerateAll() // Generates handlers, storage, CLI, client
```

**Go-Fuego**:
```go
type Device struct {
    Name     string `json:"name"`
    Location string `json:"location"`
}

fuego.Get(s, "/devices/{id}", func(c fuego.ContextWithBody[Device]) (Device, error) {
    id := c.PathParam("id")
    return loadDevice(id)
})
// OpenAPI auto-generated from signature
```

**Huma**:
```go
type Device struct {
    Name     string `json:"name" doc:"Device name"`
    Location string `json:"location" doc:"Device location"`
}

huma.Register(api, huma.Operation{
    OperationID: "get-device",
    Method:      http.MethodGet,
    Path:        "/devices/{id}",
}, func(ctx context.Context, input *struct{
    ID string `path:"id"`
}) (*struct{ Body Device }, error) {
    device := loadDevice(input.ID)
    return &struct{ Body Device }{Body: device}, nil
})
```

**Goa**:
```go
// design/design.go
var _ = Service("device", func() {
    Method("get", func() {
        Payload(func() {
            Attribute("id", String, "Device ID")
        })
        Result(Device)
        HTTP(func() {
            GET("/devices/{id}")
        })
    })
})

// Then: goa gen myapp/design
// Generates: controllers, types, OpenAPI, clients
```

---

## 🎯 Decision Matrix

### Choose **Fabrica** when:
- ✅ Building inventory/asset management systems
- ✅ Need multi-version API support (v1, v2beta1, etc.)
- ✅ Want Kubernetes-style resource management
- ✅ Need storage abstraction (file, DB, cloud)
- ✅ Require event-driven architecture (CloudEvents)
- ✅ Need reconciliation loops
- ✅ Want full-stack code generation (API + CLI + client + storage)
- ✅ Prefer template-based generation
- ✅ Building resource-centric systems

### Choose **Go-Fuego** when:
- ✅ Building modern REST APIs
- ✅ Want automatic OpenAPI from code (no YAML)
- ✅ Need net/http compatibility
- ✅ Prefer minimal boilerplate
- ✅ Starting a new API project
- ✅ Migrating from Gin/Echo
- ✅ Don't need storage/events/reconciliation
- ✅ Want a simple, intuitive framework
- ✅ Value code-first approach

### Choose **Huma** when:
- ✅ Need comprehensive OpenAPI 3.1 support
- ✅ Want router flexibility (chi, gin, fiber, etc.)
- ✅ Need multiple content types (JSON, CBOR)
- ✅ Require conditional request support
- ✅ Want automatic PATCH generation
- ✅ Building enterprise REST APIs
- ✅ Need complex validation
- ✅ Prefer schema-first approach
- ✅ Want production-proven technology

### Choose **Goa** when:
- ✅ Building microservices architectures
- ✅ Need multiple transports (HTTP + gRPC + JSON-RPC)
- ✅ Prefer design-first development
- ✅ Want comprehensive code generation (30-50% of code)
- ✅ Need strict design-implementation consistency
- ✅ Require enterprise governance
- ✅ Building complex service architectures
- ✅ Value DSL-based design
- ✅ Need multi-protocol support

---

## 🏆 Best Use Case for Each

| Framework | Sweet Spot |
|-----------|------------|
| **Fabrica** | HPC inventory, IoT device management, asset tracking, product catalogs, CMDBs |
| **Go-Fuego** | Microservice REST APIs, API gateways, backend services, public APIs |
| **Huma** | Enterprise REST APIs, multi-tenant SaaS, APIs with complex schemas |
| **Goa** | Microservices platforms, multi-protocol APIs (REST+gRPC), enterprise services |

---

## 📚 Additional Resources

### Fabrica
- GitHub: [github.com/alexlovelltroy/fabrica](https://github.com/alexlovelltroy/fabrica)
- Documentation: [docs/](/)

### Go-Fuego
- GitHub: [github.com/go-fuego/fuego](https://github.com/go-fuego/fuego)
- Documentation: [go-fuego.github.io/fuego](https://go-fuego.github.io/fuego/)
- Article: [How I write Go APIs in 2025](https://dev.to/tizzard/how-i-write-go-apis-in-2025-my-experience-with-fuego-1j5o)

### Huma
- GitHub: [github.com/danielgtaylor/huma](https://github.com/danielgtaylor/huma)
- Documentation: [huma.rocks](https://huma.rocks/)
- Tutorial: [How to Build an API with Go and Huma](https://zuplo.com/learning-center/how-to-build-an-api-with-go-and-huma)

### Goa
- GitHub: [github.com/goadesign/goa](https://github.com/goadesign/goa)
- Documentation: [goa.design](https://goa.design/)
- Blog: [Goa: Untangling Microservices](https://blog.gopheracademy.com/advent-2015/goauntanglingmicroservices/)

---

## 🤔 Still Unsure?

### Quick Decision Tree

```
Need inventory/asset management?
├─ Yes → Fabrica
└─ No
   │
   Need gRPC + HTTP from same design?
   ├─ Yes → Goa
   └─ No
      │
      Need OpenAPI 3.1 and router flexibility?
      ├─ Yes → Huma
      └─ No → Go-Fuego
```

### Can You Use Multiple?

Yes! These frameworks serve different purposes:

- **Fabrica + Huma**: Use Fabrica for inventory resources, Huma for other APIs
- **Go-Fuego + Goa**: Use Goa for complex services, Fuego for simple ones
- **Fabrica + Goa**: Use Fabrica for inventory, Goa for business logic services

---

## 📝 Conclusion

**Fabrica** is the only framework specifically designed for inventory and asset management with built-in storage, events, reconciliation, and multi-version support. If you're building an inventory system, Fabrica provides everything you need out of the box.

For general REST APIs, **Go-Fuego** offers the simplest path with automatic OpenAPI generation and minimal boilerplate.

For schema-first REST APIs with comprehensive OpenAPI 3.1 support, **Huma** provides the most flexibility and production-proven reliability.

For design-first microservices requiring multiple transports and extensive code generation, **Goa** offers unmatched capabilities with its DSL-driven approach.

Choose based on your specific needs, team preferences, and project requirements. All four frameworks are production-ready and actively maintained in 2025.
