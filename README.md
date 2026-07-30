<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica 🏗️

> Build production-ready REST APIs in Go with automatic code generation

[![REUSE status](https://api.reuse.software/badge/github.com/openchami/fabrica)](https://api.reuse.software/info/github.com/openchami/fabrica)[![golangci-lint](https://github.com/openchami/fabrica/actions/workflows/lint.yaml/badge.svg)](https://github.com/openchami/fabrica/actions/workflows/lint.yaml)
[![Build](https://github.com/openchami/fabrica/actions/workflows/release.yaml/badge.svg)](https://github.com/openchami/fabrica/actions/workflows/release.yaml)
[![Release](https://img.shields.io/github/v/release/openchami/fabrica?sort=semver)](https://github.com/openchami/fabrica/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/openchami/fabrica.svg)](https://pkg.go.dev/github.com/openchami/fabrica)
[![Go Report Card](https://goreportcard.com/badge/github.com/openchami/fabrica)](https://goreportcard.com/report/github.com/openchami/fabrica)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/OpenCHAMI/fabrica/badge)](https://securityscorecards.dev/viewer/?uri=github.com/OpenCHAMI/fabrica)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/11412/badge)](https://www.bestpractices.dev/projects/11412)

> **🏗️ Code Generator for Go REST APIs**
> Transform Go structs into production-ready REST APIs with OpenAPI specs, storage backends, and middleware in minutes.

## 📚 Documentation & Resources

| Resource | Description |
|----------|-------------|
| **[Full Documentation](https://openchami.github.io/fabrica/)** | Complete guides, tutorials, and best practices |
| **[API Reference (GoDoc)](https://pkg.go.dev/github.com/openchami/fabrica)** | Comprehensive Go package documentation |
| **[Quickstart](docs/guides/quickstart.md)** | Five minute quickstart |
| **[Getting Started Guide](docs/guides/getting-started.md)** | Step-by-step introduction to Fabrica |
| **[Examples](examples/)** | Hands-on learning with real-world projects |
| **[MCP Mode](docs/reference/mcp.md)** | Configure Fabrica as an MCP server for coding agents |


Fabrica is a powerful code generation tool that accelerates API development by transforming simple Go struct definitions into complete, production-ready REST APIs. Define your resources once, and Fabrica generates everything you need: handlers, storage layers, clients, validation, OpenAPI documentation, and more.

## ✨ Key Features

- **🚀 Zero-Config Generation** - Define resources as Go structs, get complete APIs instantly
- **📊 Multiple Storage Backends** - Choose between file-based storage or SQL databases. Annotation-driven dedicated schemas support SQLite and PostgreSQL; other Ent workflows may also use MySQL.
- **🔒 Security Ready** - Flexible middleware system for custom authentication and authorization
- **📋 OpenAPI Native** - Auto-generated specs with Swagger UI out of the box
- **🎯 Smart Validation** - Request validation with detailed, structured error responses
- **⚡ Developer Experience** - CLI tools, hot-reload development, comprehensive testing
- **📡 CloudEvents Integration** - Automatic event publishing for resource lifecycle (CRUD) and condition changes
- **🌐 Cloud-Native Ready** - [Hub/spoke API versioning](docs/guides/versioning.md), conditional requests (ETags), event-driven patterns
- **🔄 API Versioning (Hub/Spoke)** - Kubebuilder-style versioning with automatic conversion between versions
- **🏗️ Production Patterns** - Consistent API structure, error handling, and middleware
- **🔍 Built-in Client Debugging** - Generated CLI with configurable log levels for easy troubleshooting

## 🎯 Perfect For

- **Microservices Architecture** - Maintain consistent API patterns across services
- **Rapid Prototyping** - From struct definition to running API in under 5 minutes
- **API Standardization** - Enforce best practices and patterns across development teams
- **OpenAPI-First Development** - Generate comprehensive documentation alongside your code

## 📦 Installation

### Latest stable release

Install on Linux or macOS with the POSIX installer:

```bash
curl -fsSL https://github.com/openchami/fabrica/releases/latest/download/install.sh | sh
```

The default destination is `${XDG_BIN_HOME:-$HOME/.local/bin}`. The installer is non-interactive, never invokes `sudo`, and does not modify shell startup files. To inspect it before running:

```bash
curl -fsSLo install.sh https://github.com/openchami/fabrica/releases/latest/download/install.sh
less install.sh
sh install.sh
rm install.sh
```

Select a release (with or without a leading `v`) or destination through environment variables:

```bash
curl -fsSL https://github.com/openchami/fabrica/releases/latest/download/install.sh \
  | FABRICA_VERSION=v1.2.3 FABRICA_INSTALL_DIR="$HOME/bin" sh

XDG_BIN_HOME="$HOME/.local/bin" sh install.sh
```

The installer downloads the matching GoReleaser archive and `checksums.txt`, then verifies the archive's SHA-256 checksum before extraction. For higher assurance, use the inspect-before-run flow and review the release assets on GitHub.

Verify the installation:

```bash
fabrica version
```

**Using Go:**
```bash
go install github.com/openchami/fabrica/cmd/fabrica@latest
```

### Development Version

```bash
git clone https://github.com/openchami/fabrica.git
cd fabrica
make install
```

For local codegen testing against your checkout without editing a generated project's `go.mod`, run:

```bash
fabrica generate --fabrica-source /path/to/fabrica
```

Or set an environment variable for the session:

```bash
export FABRICA_SOURCE_PATH=/path/to/fabrica
fabrica generate
```

This override only affects `fabrica generate`; projects that do not opt in continue to use the released Fabrica module resolved from their own `go.mod`.

## 🤖 Using Fabrica MCP with Coding Agents

Fabrica can run as a local MCP server over stdio:

```bash
fabrica mcp --workspace /path/to/workspace
```

Use the workspace root that contains the projects the agent may inspect or modify. If `fabrica` is not on your `PATH`, replace `fabrica` with the absolute path to the binary. Mutating MCP tools default to dry-run behavior; pass `mode: "execute"` when you want the agent to apply changes.

### Claude Code

Add Fabrica as a local stdio MCP server:

```bash
claude mcp add fabrica -- fabrica mcp --workspace /path/to/workspace
```

For a project-scoped configuration, add `.mcp.json` at the project root:

```json
{
  "mcpServers": {
    "fabrica": {
      "command": "fabrica",
      "args": ["mcp", "--workspace", "/path/to/workspace"]
    }
  }
}
```

### Codex

Register the server from the Codex CLI:

```bash
codex mcp add fabrica -- fabrica mcp --workspace /path/to/workspace
```

Or add it to `~/.codex/config.toml`:

```toml
[mcp_servers.fabrica]
command = "fabrica"
args = ["mcp", "--workspace", "/path/to/workspace"]
```

After connecting, ask the agent to call `describe_workflow` with `goal: "new_crud_api"` for guided API construction, or call tools such as `inspect_project`, `validate_project`, `define_resource_schema`, `add_resource`, `generate_code`, `sync_dependencies`, and `build_project` directly.

## 📚 Learn by Example

Explore hands-on examples in the [`examples/`](examples/) directory:

- **[Basic CRUD](examples/01-basic-crud/)** ⚡ - Start here! Complete CRUD API in 5 minutes
- **[FRU Service](examples/03-fru-service/)** 🔐 - Production patterns with database integration
- **[Rack Reconciliation](examples/04-rack-reconciliation/)** 🔄 - Event-driven resource management
- **[CloudEvents Integration](examples/05-cloud-events/)** 📡 - Automatic event publishing for lifecycle and condition changes
- **[Status Subresource](examples/06-status-subresource/)** 🛡️ - Separate spec and status updates
- **[Export/Import](examples/10-export-import/)** 💾 - Offline backup and restore operations
- **[Storage Annotations](examples/12-storage-annotations/)** 🗃️ - Executable dedicated-schema example with SQLite runtime verification

---

> **🎓 Learning Path:** Start with Example 1 to understand core concepts, try Example 5 for CloudEvents, then advance to Example 3 for production patterns and database integration.

## 🏗️ Architecture Overview

Fabrica follows clean architecture principles and generates well-structured projects:

```
📁 Generated Project Structure
├── 📁 cmd/
│   ├── 📁 server/           # 🌐 REST API server with all endpoints
│   └── 📁 cli/              # 🖥️ Command-line client tools
├── 📁 pkg/
│   └── 📁 client/           # 🔌 Generated HTTP client with proper error handling
├── 📁 apis/                 # 📝 Your versioned resource types (you write these)
│   └── 📁 <group>/<version>/  # e.g., example.fabrica.dev/v1/*_types.go
├── 📁 internal/
│   ├── 📁 storage/          # 💾 Generated storage layer (file or database)
│   └── 📁 middleware/       # ⚙️ Generated middleware (auth, validation, etc.)
├── 📁 docs/                 # 📚 Generated OpenAPI specs and documentation
└── 📄 .fabrica.yaml         # ⚙️ Project configuration
```

**🏪 Storage Backends:**
- **📁 File Backend** - JSON files with atomic operations, perfect for development and small datasets
- **🗃️ Ent Backend** - Type-safe ORM with SQLite, PostgreSQL, and MySQL configuration options

### Annotation-driven dedicated Ent storage

The `+fabrica:storage=dedicated` contract is narrower than the general Ent backend. It supports PostgreSQL and SQLite; field directives require dedicated Ent storage, and dedicated mode with file storage is rejected before output. bcrypt is the only supported storage transform: required bcrypt values are enforced on create, while omitted or redacted zero values on update preserve the stored hash. Supported fields are `string`, `bool`, `int`, `int64`, `float64`, `time.Time`, and `[]string`, plus the documented scalar pointers. For sensitive updates, non-pointer sensitive zero values preserve storage; a supported non-nil pointer explicitly replaces storage, including with zero.

Resource version snapshots are file-backend only. Every Ent resource with `+fabrica:resource-versioning=enabled` is rejected before output, whether it uses generic or dedicated Ent storage. Unknown Fabrica directives fail with source-located typed parse errors rather than being ignored.

The historical annotation-layer proposal is non-authoritative and retained only as design history in [`docs/dev/ANNOTATION_LAYER_PROPOSAL.md`](docs/dev/ANNOTATION_LAYER_PROPOSAL.md). The tested package matrix, storage guides, examples, and CHANGELOG define current behavior.

Dedicated routing is exclusive, preserves the complete resource envelope, and reloads persisted resources before redacting write responses. Every generated backend compiles with a backend-common typed conflict contract; Ent constraint failures map through it to HTTP 409, while file storage remains unconstrained. Dedicated migration cursors are commit-aware: a write cursor advances only after commit and resets to the input cursor with zero copied rows on rollback.

Generated migration helpers are explicit and non-destructive: generation and server startup never invoke them, and they do not delete generic source rows. Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected for annotation-driven dedicated storage. See the [tested capability matrix](pkg/annotations/README.md) and [Executable Storage Annotations example](examples/12-storage-annotations/README.md).

### Generated request-body and compatibility contract

Generated servers apply an outer request-body limit before version negotiation, PATCH middleware, and handlers. The compatibility default is 16 MiB. Configure it globally with `--request-body-max-bytes`, the service-prefixed `*_REQUEST_BODY_MAX_BYTES` environment variable, or YAML `request_body_max_bytes`; use `request_body_max_bytes_by_resource` for a validated per-resource override keyed by the exact resource Kind. Nonpositive limits and unknown Kind keys fail startup before storage initialization.

Oversized declared or streaming bodies return HTTP 413 with `{"error":"request body too large","code":413}` and do not write storage. Bounded JSON decoding accepts exactly one JSON value, consumes trailing whitespace to verify EOF, and rejects a second value or malformed trailing bytes. PATCH storage conflicts return HTTP 409 without mutation. Generic Ent updates preserve Namespace and ResourceVersion, including explicit zero values. Both API-layout and legacy embedded-resource projects compile with file and Ent storage.

Executable evidence includes `TestGeneratedHandlers_bound_every_request_body_with_stable_413_contract`, `TestDedicatedSecurity_generated_adapter_runtime`, `TestGeneratedAnnotationProject_generic_storage_CRUD_and_queries_remain_compatible`, and `TestGeneratedLegacyHandlers_compile_for_file_and_ent_storage`.

**⚡ Generated Features:**
- ✅ REST handlers with proper HTTP methods, status codes, and content negotiation
- ✅ Comprehensive request/response validation with structured error messages
- ✅ OpenAPI 3.0 specifications with interactive Swagger UI
- ✅ Type-safe HTTP clients with automatic retries and error handling
- ✅ CLI tools for testing, administration, and automation
- ✅ Middleware for authentication, authorization, versioning, and caching

> **⚠️ IMPORTANT: Code Regeneration**
>
> Fabrica supports **regenerating code** when you modify your resources or configuration. This means:
>
> **✅ SAFE TO EDIT:**
> - `apis/<group>/<version>/*_types.go` - Your resource definitions (spec/status structs)
> - `apis.yaml` - API group and version configuration
> - `.fabrica.yaml` - Feature flags and project settings
> - `pkg/reconcilers/*_reconciler.go` (without `_generated`) - Custom reconciliation logic
> - `cmd/server/authz_classifier.go` - Authorization tuple mapping (create-once)
> - `cmd/server/openapi_extensions.go` - Custom OpenAPI routes (if you create it)
>
> **⚠️ LIMITED EDITING:**
> - `cmd/server/main.go` - Mostly generated; prefer adding custom code via helper files
>
> **❌ NEVER EDIT:**
> - **Any file ending in `_generated.go`** - These are completely regenerated on each `fabrica generate`
>
> **🔄 Regeneration Command:**
> ```bash
> fabrica generate  # Safely regenerates all *_generated.go files
> ```
>
> Your custom code in resource definitions, reconcilers, and classifier files will be preserved. All `*_generated.go` files will be completely rewritten.

## 📦 Resource Structure

Fabrica uses a **Kubernetes-inspired envelope pattern** that provides consistent structure across all resources. Every API resource follows this standardized format:

```json
{
  "apiVersion": "v1",
  "kind": "Device",
  "metadata": {
    "name": "web-server-01",
    "uid": "550e8400-e29b-41d4-a716-446655440000",
    "labels": {
      "environment": "production",
      "team": "platform"
    },
    "annotations": {
      "description": "Primary web server for customer portal"
    },
    "createdAt": "2025-10-15T10:30:00Z",
    "updatedAt": "2025-10-15T14:22:15Z"
  },
  "spec": {
    "type": "server",
    "ipAddress": "192.168.1.100",
    "status": "active",
    "port": 443,
    "tags": {"role": "web", "datacenter": "us-west-2"}
  },
  "status": {
    "health": "healthy",
    "uptime": 2592000,
    "lastChecked": "2025-10-15T14:22:15Z",
    "errorCount": 0,
    "version": "1.2.3"
  }
}
```

### 🏷️ **Envelope Components**

| Component | Purpose | Your Code | Generated |
|-----------|---------|-----------|-----------|
| **`apiVersion`** | API compatibility versioning | ❌ | ✅ Auto-managed |
| **`kind`** | Resource type identifier | ❌ | ✅ From struct name |
| **`metadata`** | Resource identity & organization | ❌ | ✅ Standard fields |
| **`spec`** | **Desired state** (your data) | ✅ **You define** | ❌ |
| **`status`** | **Observed state** (runtime info) | ✅ **You define** | ❌ |

### 📝 **What You Define**

**`spec` struct** - The desired configuration/state of your resource:
```go
type DeviceSpec struct {
    Type      string `json:"type" validate:"required,oneof=server switch router"`
    IPAddress string `json:"ipAddress" validate:"required,ip"`
    Status    string `json:"status" validate:"oneof=active inactive maintenance"`
    // ... your business logic fields
}
```

**`status` struct** - The observed/runtime state of your resource:
```go
type DeviceStatus struct {
    Health      string    `json:"health" validate:"oneof=healthy degraded unhealthy"`
    Uptime      int64     `json:"uptime"`
    LastChecked time.Time `json:"lastChecked"`
    // ... your runtime/monitoring fields
}
```

### 🎯 **Benefits of This Pattern**

- **🔄 Consistency** - All resources follow the same structure regardless of domain
- **🏷️ Rich Metadata** - Built-in support for labels, annotations, and timestamps
- **📊 State Separation** - Clear distinction between desired (`spec`) and observed (`status`) state
- **🔧 Tooling Integration** - Compatible with Kubernetes tooling and patterns
- **📈 Scalability** - Proven pattern used by Kubernetes for managing complex systems

> **💡 Pro Tip:** Focus on designing your `spec` and `status` structs - Fabrica handles all the envelope complexity automatically!


## 📖 In-Depth Documentation

**🚀 Quick Learning:**
- [Complete Getting Started Guide](docs/guides/getting-started.md) - Step-by-step tutorial
- [Quickstart Examples](examples/) - Hands-on learning with working code
- [Full Documentation Website](https://openchami.github.io/fabrica/) - All guides and tutorials

**🏗️ Architecture & Design:**
- [Architecture Overview](docs/reference/architecture.md) - Understanding Fabrica's design principles
- [Resource Model Guide](docs/guides/resource-model.md) - How to design and define resources
- [API Versioning Guide](docs/guides/versioning.md) - Hub/Spoke versioning patterns
- [API Configuration Reference](docs/apis-yaml.md) - apis.yaml structure and workflows

**💾 Storage & Data:**
- [Storage Systems](docs/guides/storage.md) - File vs database backends comparison
- [Ent Storage Integration](docs/guides/storage-ent.md) - Database setup and configuration

**⚙️ Advanced Topics:**
- [Code Generation Reference](docs/reference/codegen.md) - How templates work and customization
- [CLI Command Reference](docs/reference/cli.md) - Complete CLI documentation with flags and workflows
- [Middleware Customization](docs/guides/middleware.md) - Adding custom middleware for authentication, logging, etc.
- [Validation System](docs/guides/validation.md) - Request validation and error handling
- [Event System](docs/guides/events.md) - CloudEvents integration and event-driven patterns
- [Reconciliation](docs/guides/reconciliation.md) - Controller pattern for resource management
- [Conditional Requests & PATCH](docs/guides/conditional-and-patch.md) - ETag-based preconditions

**📖 API Documentation:**
- [GoDoc Package Reference](https://pkg.go.dev/github.com/openchami/fabrica) - Complete Go API documentation

## 🤝 Contributing

We welcome contributions from the community! Here's how to get involved:

**🐛 Report Issues:**
- [Bug Reports](https://github.com/openchami/fabrica/issues/new?template=bug_report.md)
- [Feature Requests](https://github.com/openchami/fabrica/issues/new?template=feature_request.md)

**💻 Code Contributions:**
- Fork the repository and create a feature branch
- Write tests for your changes
- Ensure all tests pass: `make test integration`
- Submit a pull request with a clear description

**💬 Community:**
- [GitHub Discussions](https://github.com/openchami/fabrica/discussions) - Ask questions and share ideas

## 🏷️ Releases & Roadmap

**Current Version:** See the [latest Fabrica release](https://github.com/openchami/fabrica/releases/latest).

**📅 Recent Updates:**
- ✅ Prometheus metrics support and generated instrumentation
- ✅ Per-resource storage schemas driven by annotations
- ✅ Generated server and client version commands
- ✅ Bulk delete support in generated clients


**📚 Resources:**
- [📋 Release Notes](https://github.com/openchami/fabrica/releases) - Detailed changelog for each version
- [ Full Changelog](CHANGELOG.md) - Complete project history

## 📄 License

This project is licensed under the [MIT License](./LICENSES/MIT.txt) - see the license file for details.
