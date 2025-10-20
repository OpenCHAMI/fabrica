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

> **🏗️ Code Generator for Go REST APIs**
> Transform Go structs into production-ready REST APIs with OpenAPI specs, storage backends, and middleware in minutes.

Fabrica is a powerful code generation tool that accelerates API development by transforming simple Go struct definitions into complete, production-ready REST APIs. Define your resources once, and Fabrica generates everything you need: handlers, storage layers, clients, validation, OpenAPI documentation, and more.

## ✨ Key Features

- **🚀 Zero-Config Generation** - Define resources as Go structs, get complete APIs instantly
- **📊 Multiple Storage Backends** - Choose between file-based storage or SQL databases (SQLite, PostgreSQL, MySQL)
- **🔒 Security Built-in** - Authentication and authorization with Casbin RBAC support
- **📋 OpenAPI Native** - Auto-generated specs with Swagger UI out of the box
- **🎯 Smart Validation** - Request validation with detailed, structured error responses
- **⚡ Developer Experience** - CLI tools, hot-reload development, comprehensive testing
- **🌐 Cloud-Native Ready** - CloudEvents, API versioning, conditional requests (ETags)
- **🏗️ Production Patterns** - Consistent API structure, error handling, and middleware

## 🎯 Perfect For

- **Microservices Architecture** - Maintain consistent API patterns across services
- **Rapid Prototyping** - From struct definition to running API in under 5 minutes
- **API Standardization** - Enforce best practices and patterns across development teams
- **OpenAPI-First Development** - Generate comprehensive documentation alongside your code

## 📦 Installation

### Latest Release (v0.2.7)

**macOS/Linux:**
```bash
# Direct download and install
curl -L https://github.com/alexlovelltroy/fabrica/releases/download/v0.2.7/fabrica-$(uname -s)-$(uname -m) -o fabrica
chmod +x fabrica
sudo mv fabrica /usr/local/bin/

# Verify installation
fabrica version
```

**Using Go:**
```bash
go install github.com/alexlovelltroy/fabrica/cmd/fabrica@v0.2.7
```

### Development Version

```bash
git clone https://github.com/alexlovelltroy/fabrica.git
cd fabrica
make install
```

## 🚀 Quick Start (5 Minutes)

**1. Initialize your project:**

```bash
fabrica init device-api
cd device-api
```

**2. Add your first resource:**

```bash
fabrica add resource Device
```

**3. Update your Spec and Status fields in `pkg/resources/device/device.go`:**

Add desired fields to generated `DeviceSpec` and `DeviceStatus` structs, retaining other code.

```go
// DeviceSpec defines the desired state of a Device
type DeviceSpec struct {
    // copy contents to generated DeviceSpec
    Type         string            `json:"type" validate:"required,oneof=server switch router storage"`
    IPAddress    string            `json:"ipAddress" validate:"required,ip"`
    Status       string            `json:"status" validate:"required,oneof=active inactive maintenance"`
    Tags         map[string]string `json:"tags,omitempty"`
    LastSeen     *time.Time        `json:"lastSeen,omitempty"`
    Port         int               `json:"port,omitempty" validate:"min=1,max=65535"`
}

// DeviceStatus represents the observed state of a Device
type DeviceStatus struct {
    // copy contents to generated DeviceSpec
    Health       string    `json:"health" validate:"required,oneof=healthy degraded unhealthy unknown"`
    Uptime       int64     `json:"uptime" validate:"min=0"`
    LastChecked  time.Time `json:"lastChecked"`
    ErrorCount   int       `json:"errorCount" validate:"min=0"`
    Version      string    `json:"version,omitempty"`
}
```

**4. Generate your API:**

```bash
fabrica generate
```

**5. Update dependencies:**

```bash
go mod tidy
```

**6. Run your server:**

```bash
go run cmd/server/main.go
```

**7. Test your API:**

```bash
# Create a device
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "web-server-01",
      "labels": {"environment": "production", "team": "platform"}
    },
    "spec": {
      "name": "web-server-01",
      "type": "server",
      "ipAddress": "192.168.1.100",
      "status": "active",
      "port": 443,
      "tags": {"role": "web", "datacenter": "us-west-2"}
    }
  }'

# List all devices
curl http://localhost:8080/devices

# Get specific device
curl http://localhost:8080/devices/web-server-01

# View OpenAPI documentation
open http://localhost:8080/swagger/
```

🎉 **That's it!** You now have a fully functional REST API with validation, OpenAPI docs, and structured error handling.

## 📚 Learn by Example

Explore hands-on examples in the [`examples/`](examples/) directory

---

> **🎓 Learning Path:** Start with Example 1 to understand core concepts, then advance to Example 3 for production patterns and database integration.

## 🏗️ Architecture Overview

Fabrica follows clean architecture principles and generates well-structured projects:

```
📁 Generated Project Structure
├── 📁 cmd/
│   ├── 📁 server/           # 🌐 REST API server with all endpoints
│   └── 📁 cli/              # 🖥️ Command-line client tools
├── 📁 pkg/
│   ├── 📁 resources/        # 📝 Your resource definitions (you write these)
│   └── 📁 client/           # 🔌 Generated HTTP client with proper error handling
├── 📁 internal/
│   ├── 📁 storage/          # 💾 Generated storage layer (file or database)
│   └── 📁 middleware/       # ⚙️ Generated middleware (auth, validation, etc.)
├── 📁 docs/                 # 📚 Generated OpenAPI specs and documentation
└── 📄 .fabrica.yaml         # ⚙️ Project configuration
```

**🏪 Storage Backends:**
- **📁 File Backend** - JSON files with atomic operations, perfect for development and small datasets
- **🗃️ Ent Backend** - Type-safe ORM supporting SQLite, PostgreSQL, MySQL for production workloads

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
> - `pkg/resources/*/` - Your resource definitions (spec/status structs)
> - `.fabrica.yaml` - Project configuration
> - `cmd/server/main.go` - Server customizations (before first `// Generated` comment)
>
> **❌ NEVER EDIT:**
> - **Any file ending in `_generated.go`** - These are completely regenerated on each `fabrica generate`
> - Files in generated directories after running `fabrica generate`
>
> **🔄 Regeneration Command:**
> ```bash
> fabrica generate  # Safely regenerates all *_generated.go files
> ```
>
> Your custom code in resource definitions and main.go will be preserved, but all generated files will be completely rewritten.

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


## 📖 Documentation

**🚀 Getting Started:**
- [Complete Getting Started Guide](docs/getting-started.md) - Step-by-step tutorial
- [Quick Start Examples](examples/) - Hands-on learning

**🏗️ Architecture & Design:**
- [Architecture Overview](docs/architecture.md) - Understanding Fabrica's design principles
- [Resource Model Guide](docs/resource-model.md) - How to design and define resources

**💾 Storage & Data:**
- [Storage Systems](docs/storage.md) - File vs database backends comparison
- [Ent Storage Integration](docs/storage-ent.md) - Database setup and configuration

**⚙️ Advanced Topics:**
- [Code Generation](docs/codegen.md) - How templates work and customization
- [Validation System](docs/validation.md) - Request validation and error handling
- [Event System](docs/events.md) - CloudEvents integration
- [Policy & Security](docs/policy-casbin.md) - Authentication and authorization

## 🤝 Contributing

We welcome contributions from the community! Here's how to get involved:

**🐛 Report Issues:**
- [Bug Reports](https://github.com/alexlovelltroy/fabrica/issues/new?template=bug_report.md)
- [Feature Requests](https://github.com/alexlovelltroy/fabrica/issues/new?template=feature_request.md)

**💻 Code Contributions:**
- Fork the repository and create a feature branch
- Write tests for your changes
- Ensure all tests pass: `make test integration`
- Submit a pull request with a clear description

**💬 Community:**
- [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions) - Ask questions and share ideas

## 🏷️ Releases & Roadmap

**Current Version:** [v0.2.7](https://github.com/alexlovelltroy/fabrica/releases/tag/v0.2.7)

**📅 Recent Updates:**
- ✅ Enhanced template system with better error handling
- ✅ Improved integration testing framework
- ✅ Updated documentation and examples
- ✅ Better CI/CD pipeline with comprehensive testing


**📚 Resources:**
- [📋 Release Notes](https://github.com/alexlovelltroy/fabrica/releases) - Detailed changelog for each version
- [ Full Changelog](CHANGELOG.md) - Complete project history

## 📄 License

This project is licensed under the [MIT License](./LICENSES/MIT.txt) - see the license file for details.
