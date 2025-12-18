<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Quick Start: Simple REST API in 30 Minutes

> **Goal:** Build and run a working REST API without learning Kubernetes concepts or advanced patterns.

This guide treats Fabrica as a **code generator** for simple CRUD APIs. We'll hide the advanced features and focus on getting you productive quickly.

## Table of Contents

- [What You'll Build](#what-youll-build)
- [Installation](#installation)
- [Step 1: Initialize Your Project](#step-1-initialize-your-project)
- [Step 2: Define Your Data](#step-2-define-your-data)
- [Step 3: Generate Code](#step-3-generate-code)
- [Step 4: Run Your API](#step-4-run-your-api)
- [Step 5: Test Your API](#step-5-test-your-api)
- [What Just Happened?](#what-just-happened)
- [Next Steps](#next-steps)

## What You'll Build

A simple REST API for managing products with these endpoints:

- `POST /products` - Create a product
- `GET /products` - List all products
- `GET /products/{id}` - Get a specific product
- `PUT /products/{id}` - Update a product
- `DELETE /products/{id}` - Delete a product

**No databases to configure.** Everything runs in-memory to keep it simple.

## Installation

### Prerequisites

- **Go 1.23+** installed ([download here](https://go.dev/dl/))
- Basic familiarity with Go syntax
- 30 minutes of your time

### Install Fabrica CLI

```bash
go install github.com/openchami/fabrica/cmd/fabrica@latest
```

Verify installation:

```bash
fabrica --version
# Output: fabrica version v0.3.1
```

## Step 1: Initialize Your Project

Create a new project with minimal complexity:

### Option A: New Directory

```bash
# Initialize simple project (creates myshop directory)
fabrica init myshop

# Enter project directory
cd myshop
```

### Option B: Existing Directory (e.g., from `gh repo create`)

If you've already created a repository with GitHub CLI or template:

```bash
# Create repo from template (example)
gh repo create myshop --template myorg/template --public
cd myshop

# Initialize Fabrica in current directory
fabrica init .
```

This will preserve existing files like `.git`, `README.md`, `LICENSE`, etc.

### Customize API Versioning (Optional)

By default, `fabrica init` creates a project with a single `v1` API version and the group `example.fabrica.dev`. You can customize this:

```bash
# Create project with custom API group
fabrica init myshop \
  --group myorg.api

# Or use interactive mode for guided setup
fabrica init myshop --interactive
```

Available init flags:
- `--group` - API group name (default: `example.fabrica.dev`)
- `--module` - Go module path
- `--validation-mode` - Validation: `strict`, `warn`, or `disabled` (default: `strict`)
- `--events` - Enable CloudEvents support
- `--storage-type` - Storage backend: `file` or `ent` (default: `file`)
- `--db` - Database driver for `ent`: `sqlite`, `postgres`, or `mysql` (default: `sqlite`)

Both options create:
- `.fabrica.yaml` with project configuration
- `apis.yaml` with API group and version configuration (default: `example.fabrica.dev` group, `v1` version)
- `go.mod` with necessary dependencies
- Basic project structure (`cmd/`, `apis/`, etc.)
- API versioning directories: `apis/example.fabrica.dev/v1/`

You'll see:

```
✓ Created .fabrica.yaml
✓ Created apis.yaml
✓ Created go.mod
✓ Created README.md (or skipped if exists)
✓ Created basic project structure

Your project is ready! Next steps:
  1. fabrica add resource Product
  2. fabrica generate
  3. go mod tidy
  4. go run ./cmd/server/
```

## Step 2: Add Your Resource

Use the Fabrica CLI to create a Product resource:

```bash
fabrica add resource Product
```

This command creates a resource definition in your project's versioned API directory. By default, resources are created in the hub (storage) version at `apis/example.fabrica.dev/v1/product_types.go`:

**Available add resource flags:**
- `--version` - Target API version (default: storage version from `apis.yaml`)
- `--force` - Force adding to non-alpha versions
- `--with-validation` - Include validation tags (default: `true`)
- `--with-status` - Include Status struct (default: `true`)

```go
package v1

import (
    "context"
    "github.com/openchami/fabrica/pkg/fabrica"
)

// Product represents a Product resource
type Product struct {
    APIVersion string           `json:"apiVersion"`
    Kind       string           `json:"kind"`
    Metadata   fabrica.Metadata `json:"metadata"`
    Spec       ProductSpec   `json:"spec" validate:"required"`
    Status     ProductStatus `json:"status,omitempty"`
}

// ProductSpec defines the desired state of Product
type ProductSpec struct {
    Name        string  `json:"name" validate:"required,min=1,max=100"`
    Description string  `json:"description,omitempty" validate:"max=500"`
    Price       float64 `json:"price" validate:"min=0"`
    InStock     bool    `json:"inStock"`
}

// ProductStatus defines the observed state of Product
type ProductStatus struct {
    Phase       string `json:"phase,omitempty"`
    Message     string `json:"message,omitempty"`
    Ready       bool   `json:"ready"`
    LastUpdated string `json:"lastUpdated,omitempty"`
}

func (r *Product) Validate(ctx context.Context) error {
    // Add custom validation logic here
    return nil
}
```

**Customize the Spec:** You can edit the fields in `ProductSpec` as needed. Your resource definitions stay in the versioned API directory (`apis/<group>/<version>/`).

## Step 3: Generate Code

Now generate the REST API handlers, storage, and routes:

```bash
fabrica generate
```

This command:
- Discovers your `Product` resource
- Generates HTTP handlers (Create, Read, Update, Delete, List)
- Generates file-based storage
- Generates API routes
- Generates OpenAPI specification

You'll see:

```
🔧 Generating code...
📦 Found 1 resource(s): Product
  ├─ Registering Product...
  ├─ Generating handlers...
  ├─ Generating storage...
  ├─ Generating OpenAPI spec...
  └─ Done!

✅ Code generation complete!
```

## Step 4: Update Dependencies

After code generation, update your Go module dependencies:

```bash
go mod tidy
```

This resolves all the new imports that were added by the code generator.

## Step 5: Run Your API

Start the server:

```bash
go run ./cmd/server/
```

You'll see:

```
Starting Fabrica server...
✓ Loaded Product handlers
✓ Registered routes
Server listening on :8080
```

Your API is now running at `http://localhost:8080`!

## Step 6: Test Your API (in a new terminal)

Open a new terminal and try the API:

### Create a Product

```bash
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "laptop-pro",
    "displayName": "MacBook Pro",
    "description": "15-inch MacBook Pro with M2 chip",
    "price": 1999.99,
    "inStock": true
  }'
```

Response:

```json
{
  "apiVersion": "v1",
  "kind": "Product",
  "metadata": {
    "name": "laptop-pro",
    "uid": "pro-abc123def456",
    "createdAt": "2025-10-15T10:30:00Z",
    "updatedAt": "2025-10-15T10:30:00Z"
  },
  "spec": {
    "name": "MacBook Pro",
    "description": "15-inch MacBook Pro with M2 chip",
    "price": 1999.99,
    "inStock": true
  },
  "status": {
    "phase": "Active",
    "ready": true,
    "lastUpdated": "2025-10-15T10:30:00Z"
  }
}
```

### Get All Products

```bash
curl http://localhost:8080/products
```

Response (flat JSON array):

```json
[
  {
    "apiVersion": "v1",
    "kind": "Product",
    "metadata": {
      "name": "laptop-pro",
      "uid": "pro-abc123def456",
      "createdAt": "2025-10-15T10:30:00Z",
      "updatedAt": "2025-10-15T10:30:00Z"
    },
    "spec": {
      "name": "MacBook Pro",
      "description": "15-inch MacBook Pro with M2 chip",
      "price": 1999.99,
      "inStock": true
    },
    "status": {
      "phase": "Active",
      "ready": true,
      "lastUpdated": "2025-10-15T10:30:00Z"
    }
  }
]
```

### Get a Specific Product

```bash
curl http://localhost:8080/products/pro-abc123def456
```

### Update a Product

```bash
curl -X PUT http://localhost:8080/products/pro-abc123def456 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "laptop-pro",
    "displayName": "MacBook Pro M3",
    "description": "Latest 15-inch MacBook Pro with M3 chip",
    "price": 2199.99,
    "inStock": true
  }'
```

### Delete a Product

```bash
curl -X DELETE http://localhost:8080/products/pro-abc123def456
```

Response:

```json
{
  "message": "Product deleted successfully"
}
```

## What Just Happened?

Let's peek under the hood (but don't worry, you don't need to edit these files):

### Generated Files

```
myshop/
├── .fabrica.yaml                      # Project configuration
├── apis.yaml                          # API group and versioning config
├── go.mod                             # Dependencies
├── README.md                          # Project README
├── apis/
│   └── example.fabrica.dev/           # API group directory
│       └── v1/
│           ├── product_types.go       # Your resource definition (you edited this)
│           └── ...
├── cmd/
│   └── server/
│       ├── main.go                    # Server entry point (with stubs)
│       ├── product_handlers_generated.go    # HTTP handlers (generated)
│       ├── routes_generated.go              # URL routing (generated)
│       ├── models_generated.go              # Server types (generated)
│       └── openapi_generated.go             # OpenAPI spec (generated)
└── internal/
    └── storage/
        └── storage_generated.go             # Storage operations (generated)
```

### What Fabrica Generated

1. **HTTP Handlers** (`cmd/server/product_handlers_generated.go`):
   - Functions to handle each REST operation (Create, Read, Update, Delete, List)
   - JSON marshaling/unmarshaling with envelope structure
   - Error handling and validation

2. **Storage Layer** (`internal/storage/storage_generated.go`):
   - File-based storage with atomic operations
   - CRUD operations for all resource types
   - List filtering and pagination support

3. **Server & Routes** (`cmd/server/routes_generated.go`):
   - URL routing configuration
   - Middleware setup for validation and versioning

4. **Client Library** (`pkg/client/client_generated.go`):
   - Go client with all operations
   - Proper error handling and retries

5. **OpenAPI Spec** (`cmd/server/openapi_generated.go`):
   - Complete API documentation
   - Swagger UI available at `/swagger/`

### What You Wrote

Just the `Product` struct definitions! That's about 20 lines of code to get a complete REST API with documentation, validation, and client libraries.

## Next Steps

### Add More Resources

Need users? Orders? Categories?

```bash
# Add a new resource type (automatically added to the hub version)
fabrica add resource Order

# Edit the generated apis/<group>/<version>/order_types.go
# Add your OrderSpec and OrderStatus fields

# Regenerate all code
fabrica generate
```

Each resource gets its own complete set of CRUD endpoints automatically.

### Add API Versions

Ready to add a new version for evolving your API?

```bash
# Step 1: Add a new API version to your project
# This creates the version directory and updates apis.yaml
fabrica add version v2

# Step 2: Add resources to the new version
# For alpha versions, resources are added without --force
fabrica add resource Device --version v2alpha1

# For stable versions, you need --force to acknowledge breaking changes
fabrica add resource Device --version v2 --force

# Step 3: Regenerate - handlers for all versions are created automatically
fabrica generate
```

**Important:** You must use `fabrica add version <version>` to create the version in `apis.yaml` before you can add resources to it. If you try to add a resource to a non-existent version, you'll get an error:

```
Error: version v2 not found in apis.yaml (available: [v1])
```

**Version naming conventions:**
- `v1alpha1`, `v2alpha1` - Alpha versions (unstable, can change freely)
- `v1beta1`, `v2beta1` - Beta versions (more stable, breaking changes discouraged)
- `v1`, `v2` - Stable versions (require `--force` flag for changes)

Fabrica uses hub/spoke versioning where all requests are converted to the hub (storage) version internally, allowing you to evolve your API gracefully.

### Add Validation

Want to validate input? Add struct tags:

```go
type ProductSpec struct {
    Name        string  `json:"name" validate:"required,min=3,max=100"`
    Description string  `json:"description" validate:"max=500"`
    Price       float64 `json:"price" validate:"required,gt=0"`
    InStock     bool    `json:"inStock"`
}
```

Validation happens automatically - invalid requests return 400 errors with detailed messages!

### Explore the API

Visit these URLs while your server is running:

- **OpenAPI Docs**: http://localhost:8080/swagger/
- **Health Check**: http://localhost:8080/health
- **API Discovery**: http://localhost:8080/api/v1/

### Learn More

When you're ready to explore more of Fabrica's capabilities:

- **[Getting Started Guide](./getting-started.md)** (Complete resource model guide)
  - Learn about labels, annotations, and metadata
  - Understand the Kubernetes-inspired resource model
  - API versioning patterns
  - Advanced features

- **[Versioning Guide](./versioning.md)** & **[apis.yaml Reference](../apis-yaml.md)**
  - Hub/spoke versioning model
  - Adding and managing API versions
  - Automatic conversion between versions

- **[Validation Guide](./validation.md)**
  - Struct tag validation
  - Custom validators
  - Detailed error responses

- **[Events Guide](./events.md)**
  - CloudEvents integration
  - Resource lifecycle events
  - Event-driven patterns

- **[Reconciliation Guide](./reconciliation.md)**
  - Implement custom controllers
  - Reconciliation loops
  - Workqueue patterns

- **[Storage Guide](./storage.md)**
  - File-based storage
  - SQL database backends (SQLite, PostgreSQL, MySQL)

### Get Help

- **Generated README**: Open `README.md` in your project
- **CLI Help**: Run `fabrica --help` or `fabrica <command> --help`
- **Documentation**: Browse `docs/` in the Fabrica repository
- **Examples**: Check `examples/` for working code samples

---

## Summary

In 30 minutes, you've:

✅ Installed Fabrica CLI
✅ Created a new project with simple mode
✅ Defined a data structure (7 lines of code)
✅ Generated a complete REST API
✅ Ran and tested your API
✅ Learned how to add more resources

**You now have a working REST API!**

When you're ready to go deeper and unlock Fabrica's full power (labels, conditions, events, reconciliation), continue to the [Resource Management Tutorial](./getting-started.md).

Happy coding! 🚀
