<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Architecture

> Understanding the framework's design, components, and extension points.

## Table of Contents

- [Overview](#overview)
- [Why Fabrica?](#why-fabrica)
- [Design Principles](#design-principles)
- [System Architecture](#system-architecture)
- [Component Overview](#component-overview)
- [Data Flow](#data-flow)
- [Extension Points](#extension-points)
- [Best Practices](#best-practices)

## Overview

Fabrica is a framework for building resource-based REST APIs with automatic code generation. It follows the Kubernetes resource pattern and emphasizes:

- **Convention over configuration** - Sensible defaults, minimal boilerplate
- **Type safety** - Compile-time checks across the stack
- **Extensibility** - Easy to customize and extend
- **Code generation** - Consistency through automation

## Why Fabrica?

### The Problem

Building REST APIs involves repetitive boilerplate:

- Define data models
- Write CRUD handlers for each resource
- Implement storage operations
- Create client libraries
- Handle versioning and migrations
- Implement authorization
- Write tests

**Result**: 90% boilerplate, 10% business logic.

### The Fabrica Solution

Define your resource once, generate everything else:

```
Resource Definition (100 lines)
    ↓
Code Generator
    ↓
Generated Code (2000+ lines)
    ├─ REST API handlers
    ├─ Storage operations
    ├─ Client library
    ├─ OpenAPI spec
    └─ CLI commands
```

**Result**: Focus on business logic, not plumbing.

### When to Use Fabrica

**Perfect for:**
- 🎯 Resource-oriented APIs (devices, products, users, etc.)
- 📊 CRUD-heavy applications
- 🏢 Internal APIs and services
- 🔧 Rapid prototyping
- 📚 Multi-version APIs

**Not ideal for:**
- ❌ Graph APIs (use GraphQL)
- ❌ RPC-style services (use gRPC)
- ❌ Real-time streaming (use WebSockets)
- ❌ Non-resource-based APIs

## Design Principles

### 1. Kubernetes-Inspired

Follow proven patterns from Kubernetes:

```go
type Resource struct {
    APIVersion string   // Version of the API
    Kind       string   // Type of resource
    Metadata   Metadata // Standard metadata
    Spec       T        // Desired state
    Status     U        // Observed state
}
```

**Why?** Kubernetes patterns are battle-tested and familiar.

### 2. Code Generation

Generate consistent code from templates:

```
Templates (Manual)
    ↓
Generator Engine
    ↓
Generated Code (Automatic)
```

**Why?** One source of truth, applied everywhere.

### 3. Type Safety

Compile-time checking across the stack:

```go
// Server side
func CreateDevice(device *Device) error { ... }

// Storage layer
storage.Save(ctx, device) // Type-checked

// Client side
client.CreateDevice(ctx, device) // Type-checked
```

**Why?** Catch errors at compile time, not runtime.

### 4. Pluggable Everything

Interface-based design for flexibility:

- **Storage**: File, database, cloud
- **Authorization**: RBAC, ABAC, custom
- **Versioning**: Single, multi-version
- **Transport**: HTTP, gRPC (future)

**Why?** Adapt to your needs without framework changes.

### 5. Progressive Enhancement

Start simple, add features as needed:

```
1. Define resource        → Basic CRUD
2. Add labels            → Query and filter
3. Add authorization     → Access control
4. Add versioning        → Compatibility
5. Add custom storage    → Scale
```

**Why?** Don't pay for features you don't use.

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────┐
│                   HTTP Layer                        │
│  ┌────────────────────────────────────────────┐    │
│  │   Generated REST API Handlers              │    │
│  │   (List, Get, Create, Update, Delete)      │    │
│  └────────────────┬───────────────────────────┘    │
└───────────────────┼────────────────────────────────┘
                    │
┌───────────────────▼────────────────────────────────┐
│              Framework Layer                        │
│  ┌─────────────┐ ┌──────────────┐ ┌─────────────┐ │
│  │ Versioning  │ │ Authorization│ │ Validation  │ │
│  │  Registry   │ │   Policies   │ │   Rules     │ │
│  └─────────────┘ └──────────────┘ └─────────────┘ │
└───────────────────┬────────────────────────────────┘
                    │
┌───────────────────▼────────────────────────────────┐
│              Storage Layer                          │
│  ┌────────────────────────────────────────────┐    │
│  │   Storage Backend Interface                │    │
│  │   ┌──────────┐ ┌──────────┐ ┌──────────┐  │    │
│  │   │   File   │ │ Database │ │  Custom  │  │    │
│  │   │ Backend  │ │ Backend  │ │ Backend  │  │    │
│  │   └──────────┘ └──────────┘ └──────────┘  │    │
│  └────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘
```

### Component Layers

**Layer 1: HTTP Layer**
- Generated REST API handlers
- Route registration
- Request/response serialization
- HTTP error handling

**Layer 2: Framework Layer**
- Version negotiation and conversion
- Authorization policy enforcement
- Resource validation
- UID generation

**Layer 3: Storage Layer**
- Pluggable storage backends
- CRUD operations
- Transaction support (backend-dependent)
- Query optimization

## Component Overview

### 1. Resource Model (`pkg/resource`)

**Purpose**: Define resource structure and common operations.

**Key Components:**
- `Resource` struct - Base resource type
- `Metadata` - Name, UID, labels, annotations, timestamps
- `Conditions` - Status conditions pattern
- UID generation - Structured identifiers

**Example:**
```go
type Device struct {
    APIVersion string       `json:"apiVersion"`
    Kind       string       `json:"kind"`
    Metadata   Metadata     `json:"metadata"`
    Spec       DeviceSpec   `json:"spec"`
    Status     DeviceStatus `json:"status,omitempty"`
}
```

### 2. Code Generator (`pkg/codegen`)

**Purpose**: Generate consistent code from resource definitions.

**Key Components:**
- `Generator` - Main code generation engine
- `ResourceMetadata` - Extracted resource information
- `Templates` - Go text templates
- Template functions - Helper functions (camelCase, toLower, etc.)

**Flow:**
```
Resource Definition
    ↓
Reflection (extract metadata)
    ↓
Template Application
    ↓
Go code formatting
    ↓
File writing
```

### 3. Storage System (`pkg/storage`)

**Purpose**: Pluggable persistence layer.

**Key Components:**
- `StorageBackend` interface - Core operations
- `FileBackend` - File-based implementation
- `ResourceStorage[T]` - Type-safe wrapper
- Error types - ErrNotFound, ErrAlreadyExists, etc.

**Operations:**
```go
backend.LoadAll(ctx, "Device")       // List all
backend.Load(ctx, "Device", uid)     // Get one
backend.Save(ctx, "Device", uid, data) // Create/Update
backend.Delete(ctx, "Device", uid)   // Delete
backend.Exists(ctx, "Device", uid)   // Check existence
```

### 4. Versioning (`pkg/versioning`)

**Purpose**: Multi-version schema support.

**Key Components:**
- `VersionRegistry` - Register and lookup versions
- `SchemaVersion` - Version metadata
- `VersionConverter` - Convert between versions
- Middleware - HTTP version negotiation

**Flow:**
```
Client Request (v1)
    ↓
Version Registry (lookup v1)
    ↓
Storage (load v2)
    ↓
Converter (v2 → v1)
    ↓
Response (v1)
```

### 5. Authentication & Authorization (User-Implemented)

**Purpose**: Flexible access control through custom middleware.

**Key Points:**
- Authentication and authorization are **not generated** by Fabrica
- You implement them as custom middleware (see [Middleware Guide](../guides/middleware.md))
- Common patterns: JWT validation, RBAC, ABAC, TokenSmith integration
- Middleware order matters: auth before resource handlers

**Example Authentication Middleware:**
```go
// internal/middleware/auth.go
package middleware

import (
    "context"
    "net/http"
    "strings"
    "github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                return jwtSecret, nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            // Add claims to context for downstream handlers
            ctx := context.WithValue(r.Context(), "user", token.Claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**Integration:**
```go
// cmd/server/main.go
func main() {
    router := chi.NewRouter()

    // Add your auth middleware
    router.Use(AuthMiddleware(jwtSecret))

    // Register generated routes
    RegisterRoutes(router, storage, eventBus)

    http.ListenAndServe(":8080", router)
}
```

**See Also:**
- [Middleware Customization Guide](../guides/middleware.md) - How to add custom middleware
- Examples: JWT validation, TokenSmith, role-based access control

## Data Flow

### Create Resource Flow

```
1. HTTP POST /devices
    ↓
2. Auth Middleware (if configured)
    ↓
3. Generated Handler: CreateDevice()
    ↓
4. Validation Middleware
    ↓
5. Generate UID: dev-1a2b3c4d
    ↓
6. Set Timestamps: CreatedAt, UpdatedAt
    ↓
7. Storage: backend.Save()
    ↓
8. Event Publishing (if enabled)
    ↓
9. Response: 201 Created with resource
```

### Get Resource Flow

```
1. HTTP GET /devices/dev-123
    ↓
2. Auth Middleware (if configured)
    ↓
3. Generated Handler: GetDevice()
    ↓
4. Version Negotiation: Body `apiVersion`, URL version, or Accept header
    ↓
5. Storage: backend.Load()
    ↓
6. Version Conversion: v2 → v1 (if needed)
    ↓
7. Conditional Middleware: Check ETags
    ↓
8. Response: 200 OK with resource
```

### List Resources Flow

```
1. HTTP GET /devices
    ↓
2. Auth Middleware (if configured)
    ↓
3. Generated Handler: ListDevices()
    ↓
4. Storage: backend.LoadAll()
    ↓
5. Label Filtering: (if query params)
    ↓
6. Version Conversion: (if needed)
    ↓
7. Response: 200 OK with array
```

## Extension Points

### 1. Custom Storage Backend

Implement `StorageBackend` interface:

```go
type PostgresBackend struct {
    db *sql.DB
}

func (b *PostgresBackend) Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error) {
    var data json.RawMessage
    err := b.db.QueryRowContext(ctx,
        "SELECT data FROM resources WHERE type=$1 AND uid=$2",
        resourceType, uid,
    ).Scan(&data)
    return data, err
}

// Implement other methods...
```

### 2. Custom Middleware

Add authentication or authorization as middleware:

```go
// internal/middleware/tenant.go
package middleware

import (
    "context"
    "net/http"
)

type TenantKey string

const TenantContextKey = TenantKey("tenant")

func MultiTenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract tenant from JWT claims (assumed set by auth middleware)
        user, _ := r.Context().Value("user").(map[string]interface{})
        tenantID, _ := user["tenant_id"].(string)

        if tenantID == "" {
            http.Error(w, "Missing tenant information", http.StatusBadRequest)
            return
        }

        // Add tenant to context for handlers
        ctx := context.WithValue(r.Context(), TenantContextKey, tenantID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Usage in handlers:**
```go
// Filter resources by tenant
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
    tenantID := r.Context().Value(middleware.TenantContextKey).(string)

    devices, _ := h.Storage.LoadAll(r.Context(), "Device")

    // Filter by tenant label
    filtered := []Device{}
    for _, device := range devices {
        if device.Metadata.Labels["tenant"] == tenantID {
            filtered = append(filtered, device)
        }
    }

    json.NewEncoder(w).Encode(filtered)
}
```

**See:** [Middleware Customization Guide](../guides/middleware.md)

### 3. Custom Template Functions

Add functions to code generator:

```go
generator.Templates["handlers"].Funcs(template.FuncMap{
    "snakeCase": func(s string) string {
        // Convert to snake_case
        return strings.ToLower(regexp.MustCompile(`([A-Z])`).
            ReplaceAllString(s, "_$1"))
    },
})
```

### 4. Middleware Integration

Add middleware to generated routes:

```go
// In your main.go
func main() {
    backend := storage.NewFileBackend("./data")

    // Register routes
    RegisterRoutes(backend)

    // Add middleware
    handler := loggingMiddleware(
        authMiddleware(
            http.DefaultServeMux,
        ),
    )

    http.ListenAndServe(":8080", handler)
}
```

### 5. Custom Validation

Add validation to resource:

```go
type Device struct {
    APIVersion string     `json:"apiVersion"`
    Kind       string     `json:"kind"`
    Metadata   Metadata   `json:"metadata"`
    Spec       DeviceSpec `json:"spec"`
}

func (d *Device) Validate() error {
    if d.Spec.Name == "" {
        return fmt.Errorf("name is required")
    }
    if d.Spec.Type != "sensor" && d.Spec.Type != "actuator" {
        return fmt.Errorf("invalid device type")
    }
    return nil
}
```

Call in handler:

```go
func CreateDevice(device *Device) error {
    if err := device.Validate(); err != nil {
        return err
    }
    // Continue with save...
}
```

## Best Practices

### Resource Design

**DO:**
- ✅ Keep Spec immutable (desired state)
- ✅ Use Status for observed state
- ✅ Add comprehensive labels
- ✅ Use structured UIDs (prefix-hex)
- ✅ Include timestamps

**DON'T:**
- ❌ Mix Spec and Status concerns
- ❌ Store computed values in Spec
- ❌ Use UUIDs (use structured UIDs)
- ❌ Forget to register UID prefix

### Code Generation

**DO:**
- ✅ Version control templates
- ✅ Document template customizations
- ✅ Test generated code
- ✅ Use template functions
- ✅ Generate before commit

**DON'T:**
- ❌ Edit generated files directly
- ❌ Mix manual and generated code
- ❌ Skip code generation step
- ❌ Commit without regenerating

### Storage

**DO:**
- ✅ Use context for timeouts
- ✅ Handle ErrNotFound explicitly
- ✅ Implement transaction support
- ✅ Add proper indexes
- ✅ Cache when appropriate

**DON'T:**
- ❌ Ignore context cancellation
- ❌ Load all resources in memory
- ❌ Skip error handling
- ❌ Block on storage operations

### Authorization

**DO:**
- ✅ Start with RBAC
- ✅ Add ABAC as needed
- ✅ Test all policy paths
- ✅ Document authorization rules
- ✅ Use JWT claims

**DON'T:**
- ❌ Hardcode permissions
- ❌ Skip authorization checks
- ❌ Trust client-provided data
- ❌ Implement security through obscurity

### Versioning

**DO:**
- ✅ Use semantic versions (v1, v2, v3)
- ✅ Mark stability (alpha, beta, stable)
- ✅ Provide bidirectional conversion
- ✅ Deprecate versions gracefully
- ✅ Document breaking changes

**DON'T:**
- ❌ Break existing versions
- ❌ Skip version conversion testing
- ❌ Remove versions without deprecation period
- ❌ Use arbitrary version strings

## Summary

Fabrica provides:

- 🏗️ **Solid foundation** - Kubernetes-inspired patterns
- 🚀 **Rapid development** - Code generation reduces boilerplate
- 🔒 **Type safety** - Compile-time checks everywhere
- 🔌 **Flexibility** - Pluggable components
- 📚 **Scalability** - Multi-version support built-in

**Next Steps:**

- Learn the [Resource Model](../guides/resource-model.md)
- Understand [Code Generation](codegen.md)
- Explore [Storage Options](../guides/storage.md)
- Implement [Custom Middleware](../guides/middleware.md) for authentication/authorization

---

**Questions?** [Open an Issue](https://github.com/openchami/fabrica/issues) | **Want to contribute?** [Contributing Guide](../../CONTRIBUTING.md)
