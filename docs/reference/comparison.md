<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Go Framework Comparison: Finding the Right Tool for Your Project

This document provides an honest, balanced comparison of Go frameworks that generate code or provide comprehensive API development features. For simple routing frameworks (Chi, Gin, Fiber, Echo), see their respective documentation—they're excellent but serve a different purpose than the frameworks compared here.

## 📊 Framework Categories

These frameworks go beyond simple routing to provide higher-level abstractions:

### 1. **Code Generation Frameworks**
Generate significant portions of your codebase from specifications or definitions.
- **Goa** - Design-first with DSL, generates full stack from design
- **Fabrica** - Resource-centric, template-based generation
- **Buffalo** - Rails-like full-stack framework with scaffolding

### 2. **OpenAPI-First Frameworks**
Prioritize comprehensive OpenAPI documentation and schema-driven development.
- **Huma** - Schema-first with comprehensive OpenAPI 3.1 and advanced features
- **Go-Fuego** - Code-first with automatic OpenAPI 3.0 generation
- **Ogen** - Generate server/client from existing OpenAPI specs

### 3. **Full-Stack MVC Frameworks**
Complete frameworks with ORM, templating, and batteries included.
- **Beego** - Enterprise-focused MVC with comprehensive tooling
- **Buffalo** - Rails-like rapid development framework
- **Revel** - Full-stack with hot reload

---

## 🎯 Quick Decision Guide

**I need...**

- **Generate code from OpenAPI spec** → Ogen
- **Generate OpenAPI 3.1 with JSON Patch** → Huma
- **Generate OpenAPI 3.0 easily** → Go-Fuego
- **Manage inventory/resources with storage** → Fabrica
- **Design-first microservices (HTTP+gRPC)** → Goa
- **Full-stack web app** → Buffalo, Beego
- **Multi-version API support** → Fabrica, Huma (manual)
- **Enterprise MVC** → Beego
- **Just need a simple REST API** → Use Chi, Gin, or Echo (different category)

---

## 🔍 Detailed Framework Profiles

### Fabrica

**What it is**: Resource-centric inventory management framework with comprehensive code generation

**Philosophy**: Kubernetes-style resource management with template-based full-stack generation

**Strengths**:
- ✅ Built-in storage abstraction (file, database, cloud)
- ✅ CloudEvents-compliant event system
- ✅ Kubernetes-style reconciliation loops
- ✅ Native multi-version schema support with automatic conversion
- ✅ Template-based full-stack generation (API + CLI + client + storage)
- ✅ Resource model with labels, annotations, conditions
- ✅ Policy-based authorization framework
- ✅ **Built-in conditional requests (ETags, If-Match, If-None-Match)**
- ✅ **Comprehensive PATCH support (RFC 7386, RFC 6902, shorthand)**
- ✅ **Optimistic concurrency control with ETags**
- ✅ **Comprehensive validation (struct tags + K8s-style validators)**

**Weaknesses**:
- ❌ Opinionated resource structure (not suitable for all APIs)
- ❌ Learning curve for resource model concepts
- ❌ Inventory/asset domain focus (less suitable for general APIs)
- ❌ Smaller community compared to other frameworks
- ❌ More complex setup than simpler alternatives

**Best For**:
- Inventory management (IT assets, devices, IoT, products)
- Systems needing resource versioning (v1, v2beta1)
- Event-driven architectures with reconciliation
- Projects wanting Kubernetes-style patterns
- Asset tracking and management systems

**Not For**:
- Simple CRUD APIs (use Huma or Go-Fuego)
- Non-resource-based systems
- Projects needing maximum flexibility
- Teams unfamiliar with Kubernetes concepts
- General-purpose REST APIs

**Production Use**: OpenCHAMI HPC inventory management

---

### Huma

**What it is**: Schema-first REST/RPC framework with comprehensive OpenAPI 3.1

**Philosophy**: Type-safe, schema-driven development with extensive built-in features

**Strengths**:
- ✅ Full OpenAPI 3.1 and JSON Schema support
- ✅ **Built-in JSON Patch and JSON Merge Patch** (RFC 7396, RFC 6902)
- ✅ Router-agnostic (works with any Go router)
- ✅ Multiple content types (JSON, CBOR) with compression (gzip, Brotli)
- ✅ Conditional requests (If-Match, If-Unmodified-Since, ETags)
- ✅ Automatic PATCH generation from GET+PUT
- ✅ Production-proven (millions of users in live streaming)
- ✅ Excellent validation with detailed error messages
- ✅ Beautiful documentation (Stoplight Elements integration)
- ✅ Negotiated response transformations

**Weaknesses**:
- ❌ No code generation beyond OpenAPI
- ❌ Manual versioning (no automatic conversion)
- ❌ No storage abstraction
- ❌ Verbose API for simple use cases
- ❌ No built-in event system
- ❌ No reconciliation framework

**Best For**:
- Enterprise REST APIs with complex schemas
- APIs needing comprehensive OpenAPI 3.1
- Projects requiring PATCH operations
- Multi-tenant SaaS platforms
- APIs with strict validation requirements
- Teams wanting router flexibility

**Not For**:
- Projects needing code generation
- Inventory/resource management (use Fabrica)
- gRPC services (use Goa)
- Full-stack web apps (use Buffalo)

**Production Use**: Live streaming platforms, enterprise SaaS, high-scale APIs

---

### Go-Fuego

**What it is**: Modern code-first framework with automatic OpenAPI generation

**Philosophy**: Minimal boilerplate with modern Go idioms and automatic documentation

**Strengths**:
- ✅ Zero YAML - OpenAPI 3.0 from Go code automatically
- ✅ Built on Go 1.22+ stdlib (no lock-in)
- ✅ Uses generics for type safety
- ✅ Very low boilerplate
- ✅ Easy migration from Gin/Echo
- ✅ Built-in validation (go-playground/validator)
- ✅ Simple, intuitive API
- ✅ Can plugin to existing Gin/Echo servers

**Weaknesses**:
- ❌ OpenAPI 3.0 only (not 3.1)
- ❌ No code generation (only OpenAPI)
- ❌ No storage abstraction
- ❌ No built-in versioning
- ❌ No JSON Patch support
- ❌ Newer framework (smaller community)
- ❌ No event system or reconciliation

**Best For**:
- Modern REST APIs with OpenAPI needs
- Teams wanting code-first OpenAPI
- Projects valuing stdlib compatibility
- Microservices
- Migrating from Gin/Echo to OpenAPI

**Not For**:
- Projects needing OpenAPI 3.1
- Full-stack generation needs
- Complex enterprise requirements
- Inventory management (use Fabrica)

**Production Use**: Modern microservices, API-first applications

---

### Goa

**What it is**: Design-first framework with DSL-driven comprehensive code generation

**Philosophy**: Design your API contract first in a DSL, generate everything from it

**Strengths**:
- ✅ Elegant, type-safe DSL for API design
- ✅ Generates 30-50% of codebase automatically
- ✅ Multi-transport (HTTP, gRPC, JSON-RPC 2.0)
- ✅ WebSocket and SSE streaming support
- ✅ Zero drift between design and code
- ✅ AI-powered design wizard (new in 2025)
- ✅ Complete tooling (server, client, CLI, OpenAPI, Protocol Buffers)
- ✅ Strong type safety throughout

**Weaknesses**:
- ❌ High learning curve (custom DSL to learn)
- ❌ Less flexibility in generated code structure
- ❌ Opinionated architecture
- ❌ No storage abstraction
- ❌ Debugging generated code can be complex
- ❌ No built-in event system or reconciliation

**Best For**:
- Microservices architectures
- Multi-protocol APIs (REST + gRPC from same design)
- Teams valuing design governance
- Enterprise service architectures
- Regulated industries requiring design contracts

**Not For**:
- Simple REST APIs
- Teams unfamiliar with DSLs
- Rapid prototyping
- Projects needing storage/events (use Fabrica)

**Production Use**: Enterprise microservices, financial services, regulated industries

---

### Buffalo

**What it is**: Rails-like full-stack web development framework

**Philosophy**: Convention over configuration for rapid development

**Strengths**:
- ✅ Complete full-stack solution (backend + frontend)
- ✅ Built-in ORM (Pop) with migrations
- ✅ Asset pipeline for frontend
- ✅ Hot reload during development
- ✅ Scaffolding generators
- ✅ WebSocket support
- ✅ Session management
- ✅ Task runners

**Weaknesses**:
- ❌ Heavy and opinionated
- ❌ Learning curve for full ecosystem
- ❌ Slower development pace recently
- ❌ Less suitable for API-only projects
- ❌ No OpenAPI generation
- ❌ Overkill for microservices

**Best For**:
- Full-stack web applications
- Teams from Rails/Django background
- Rapid prototyping of web apps
- Traditional web applications with server-side rendering

**Not For**:
- Microservices
- API-only projects
- Teams wanting minimal dependencies
- Projects requiring OpenAPI

**Production Use**: Full-stack web applications, startups, rapid MVPs

---

### Beego

**What it is**: Enterprise MVC framework with comprehensive features

**Philosophy**: Complete framework for large enterprise applications

**Strengths**:
- ✅ Full MVC architecture
- ✅ Built-in ORM with query builder
- ✅ Admin dashboard generation
- ✅ Task scheduling and cron jobs
- ✅ I18n/L10n support
- ✅ Logging, caching, session management
- ✅ Swagger integration
- ✅ Namespace routing

**Weaknesses**:
- ❌ Heavy and complex
- ❌ Older design patterns
- ❌ Less active development recently
- ❌ Steep learning curve
- ❌ Not suitable for microservices
- ❌ Less idiomatic Go

**Best For**:
- Large enterprise applications
- Teams wanting complete framework
- Traditional MVC projects
- Applications needing admin interfaces

**Not For**:
- Microservices
- Modern Go idioms
- Simple APIs
- Cloud-native applications

**Production Use**: Enterprise web applications, admin panels

---

### Ogen

**What it is**: Code generator that creates type-safe server and client from OpenAPI specs

**Philosophy**: OpenAPI specification as the source of truth

**Strengths**:
- ✅ Generates both server and client code
- ✅ Full type safety from OpenAPI spec
- ✅ Supports OpenAPI 3.0 and 3.1
- ✅ No reflection at runtime
- ✅ Fast code generation
- ✅ Minimal dependencies

**Weaknesses**:
- ❌ Requires existing OpenAPI specification
- ❌ Generated code can be verbose
- ❌ Limited customization of generated code
- ❌ No framework features (just generation)
- ❌ Must maintain separate OpenAPI spec file

**Best For**:
- API-first development with existing specs
- Teams with OpenAPI specs from other tools
- Projects requiring strict OpenAPI compliance

**Not For**:
- Starting from scratch (use Huma or Go-Fuego)
- Projects wanting framework features
- Teams unfamiliar with OpenAPI

---

## 📊 Feature Comparison Matrix

| Feature | Fabrica | Huma | Go-Fuego | Goa | Buffalo | Beego | Ogen |
|---------|---------|------|----------|-----|---------|-------|------|
| **OpenAPI Generation** | Template | 3.1 | 3.0 | Yes | No | Limited | From spec |
| **Code Generation** | Full stack | No | No | Full stack | Scaffolding | No | Server+Client |
| **Storage Abstraction** | Yes | No | No | No | ORM | ORM | No |
| **Event System** | CloudEvents | No | No | No | No | No | No |
| **Reconciliation** | K8s-style | No | No | No | No | No | No |
| **Versioning** | Built-in | Manual | Manual | DSL | Manual | Manual | Via spec |
| **Validation** | **K8s + Tags** | **Built-in** | **Built-in** | DSL | Manual | Manual | Spec-based |
| **JSON Patch** | **Yes** | **Yes** | No | No | No | No | Spec-based |
| **Conditional Requests** | **Yes** | **Yes** | No | No | No | No | Spec-based |
| **Multi-transport** | HTTP | HTTP | HTTP | HTTP/gRPC/JSON-RPC | HTTP | HTTP | HTTP |
| **Learning Curve** | Medium | Medium | Low | High | High | High | Medium |
| **Best For** | Inventory | Enterprise API | Modern API | Microservices | Full-stack | Enterprise MVC | Spec-first |

---

## 🎭 Honest Strengths & Weaknesses

### What Fabrica Does Better
- **Only framework** with built-in resource storage abstraction
- **Only framework** with CloudEvents event system
- **Only framework** with Kubernetes-style reconciliation
- **Best** native multi-version schema support with automatic conversion
- **Best** for inventory/asset management domain
- **Most comprehensive** for resource-centric systems

### What Fabrica Does Worse
- **More complex** than focused frameworks
- **More opinionated** than flexible alternatives
- **Smaller community** than established frameworks
- **Less suitable** for non-resource-based APIs
- **Fewer production references** than older frameworks
- **Steeper learning curve** than Go-Fuego

### What Each Framework Does Best

**Huma**: Most comprehensive OpenAPI 3.1, built-in PATCH/validation, router-agnostic, production-proven
**Go-Fuego**: Simplest modern code-first OpenAPI generation with minimal boilerplate
**Goa**: Best design-first approach, multi-protocol support, comprehensive generation
**Buffalo**: Best Rails-like full-stack experience with complete tooling
**Beego**: Most complete enterprise MVC features and admin tools
**Ogen**: Best for strict OpenAPI spec compliance and type safety

---

## 🤔 Choosing the Right Framework

### Decision Tree

```
What are you building?

├─ Inventory/Asset Management System?
│  └─ Need storage + events + reconciliation?
│     ├─ Yes → Fabrica
│     └─ No → Huma (if complex) or Go-Fuego (if simple)
│
├─ Microservices needing HTTP + gRPC?
│  └─ Goa
│
├─ REST API with comprehensive OpenAPI 3.1?
│  ├─ Need JSON Patch + advanced features?
│  │  └─ Huma
│  ├─ Want simple code-first?
│  │  └─ Go-Fuego
│  └─ Have existing OpenAPI spec?
│     └─ Ogen
│
├─ Full-Stack Web Application?
│  ├─ Want Rails-like experience?
│  │  └─ Buffalo
│  └─ Need enterprise MVC?
│     └─ Beego
│
└─ Simple REST API without special features?
   └─ Use Chi, Gin, or Echo (lightweight routers)
```

### By Use Case

| Use Case | Best Choice | Alternative |
|----------|-------------|-------------|
| HPC/IoT Inventory | Fabrica | Huma + custom storage |
| Enterprise REST API | Huma | Goa |
| Simple Modern API | Go-Fuego | Huma |
| Multi-protocol Microservices | Goa | Separate services |
| Full-Stack Web App | Buffalo | Beego |
| Enterprise MVC | Beego | Buffalo |
| API-First (existing spec) | Ogen | Huma |
| Resource Management | Fabrica | Custom solution |

---

## 💡 Can You Mix Frameworks?

**Yes!** Common patterns:

- **Fabrica + Huma**: Use Fabrica for inventory resources, Huma for other APIs
- **Goa + Go-Fuego**: Goa for main services, Go-Fuego for utility endpoints
- **Buffalo + API framework**: Buffalo for web UI, separate API microservice
- **Multiple services**: Different frameworks for different microservices based on needs

---

## 🎓 Learning Resources

### Fabrica
- **Docs**: [github.com/openchami/fabrica](https://github.com/openchami/fabrica)
- **Best for**: Inventory and asset management systems

### Huma
- **Docs**: [huma.rocks](https://huma.rocks/)
- **Tutorial**: [Building APIs with Huma](https://huma.rocks/tutorial/installation/)
- **Best for**: Enterprise REST APIs with OpenAPI 3.1

### Go-Fuego
- **Docs**: [go-fuego.github.io/fuego](https://go-fuego.github.io/fuego/)
- **Article**: [How I write Go APIs in 2025](https://dev.to/tizzard/how-i-write-go-apis-in-2025-my-experience-with-fuego-1j5o)
- **Best for**: Modern REST APIs with simple OpenAPI

### Goa
- **Docs**: [goa.design](https://goa.design/)
- **Tutorial**: [Getting Started](https://goa.design/docs/1-goa/quickstart/)
- **Best for**: Design-first microservices

### Buffalo
- **Docs**: [gobuffalo.io](https://gobuffalo.io/)
- **Best for**: Full-stack web applications

### Beego
- **Docs**: [Beego Wiki](https://github.com/beego/beego/wiki)
- **Best for**: Enterprise MVC applications

### Ogen
- **Docs**: [ogen.dev](https://ogen.dev/)
- **Best for**: OpenAPI spec-first development

---

## 📝 Final Thoughts

**There is no "best" framework** - only the best framework for your specific needs.

### Choose based on:

1. **Project requirements** (features, OpenAPI version, transport protocols)
2. **Team experience** (Rails → Buffalo, Kubernetes → Fabrica, DSL comfort → Goa)
3. **Domain fit** (inventory → Fabrica, general API → Huma/Go-Fuego, enterprise → Beego)
4. **Complexity tolerance** (simple → Go-Fuego, complex → Goa/Fabrica)
5. **Long-term maintenance** (community size, update frequency, stability)

### Remember:

- **Huma**: Production-proven, most complete OpenAPI 3.1, excellent for enterprise APIs
- **Go-Fuego**: Modern and simple, great for new projects with OpenAPI needs
- **Goa**: Powerful design-first, best for multi-protocol microservices
- **Fabrica**: Specialized for inventory/assets, excellent for that specific domain
- **Buffalo/Beego**: Full-stack frameworks, good for traditional web apps
- **Ogen**: Best when you already have OpenAPI specs

### Don't overthink it:

- **For most REST APIs**: Start with **Huma** or **Go-Fuego**
- **Need simple routing only**: Use **Chi** or **Gin** (different category)
- **Building inventory system**: Consider **Fabrica**
- **Need HTTP + gRPC**: Use **Goa**
- **Full-stack web app**: Use **Buffalo**

All frameworks mentioned here are production-ready. Pick one that fits your needs and build something great! 🚀

---

## 🔗 Related Comparisons

For lightweight routing frameworks (Chi, Gin, Fiber, Echo), see:
- [Top Go Frameworks 2025 - LogRocket](https://blog.logrocket.com/top-go-frameworks-2025/)
- [Go Web Framework Comparison](https://github.com/mingrammer/go-web-framework-stars)

Those frameworks are excellent for simple routing but are fundamentally different from the code-generation and comprehensive frameworks compared here.
