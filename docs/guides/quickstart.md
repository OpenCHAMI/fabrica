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
go install github.com/alexlovelltroy/fabrica/cmd/fabrica@latest
```

Verify installation:

```bash
fabrica --version
# Output: fabrica version v0.2.8
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

Both options create:
- `.fabrica.yaml` with project configuration
- `go.mod` with necessary dependencies
- Basic project structure (`cmd/`, `pkg/`, etc.)

You'll see:

```
✓ Created .fabrica.yaml
✓ Created go.mod
✓ Created README.md (or skipped if exists)
✓ Created basic project structure

Your project is ready! Next steps:
  1. fabrica add resource Product
  2. fabrica generate
  3. go mod tidy
  4. go run cmd/server/main.go
```

## Step 2: Add Your Resource

Use the Fabrica CLI to create a Product resource:

```bash
fabrica add resource Product
```

This command creates a resource definition at `pkg/resources/product/product.go`:

```go
package product

import (
    "github.com/alexlovelltroy/fabrica/pkg/resource"
)

// Product represents a Product resource
type Product struct {
    resource.Resource `json:",inline"`
    Spec              ProductSpec   `json:"spec"`
    Status            ProductStatus `json:"status"`
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
```

**Customize the Spec:** You can edit the fields in `ProductSpec` as needed:

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
- Updates server registration

## Step 4: Update Dependencies

After code generation, update your Go module dependencies:

```bash
go mod tidy
```

This resolves all the new imports that were added by the code generator.

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

## Step 5: Run Your API

Start the server:

```bash
go run cmd/server/main.go
```

You'll see:

```
Starting Fabrica server...
✓ Loaded Product handlers
✓ Registered routes
Server listening on :8080
```

Your API is now running at `http://localhost:8080`!

## Step 6: Test Your API

Open a new terminal and try the API:

### Create a Product

```bash
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {
      "name": "laptop-pro"
    },
    "spec": {
      "name": "MacBook Pro",
      "description": "15-inch MacBook Pro with M2 chip",
      "price": 1999.99,
      "inStock": true
    }
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

Response:

```json
{
  "items": [
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
}
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
    "metadata": {
      "name": "laptop-pro"
    },
    "spec": {
      "name": "MacBook Pro M3",
      "description": "Latest 15-inch MacBook Pro with M3 chip",
      "price": 2199.99,
      "inStock": true
    }
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
├── .fabrica.yaml                   # Project configuration
├── go.mod                          # Dependencies
├── README.md                       # Project README
├── pkg/
│   └── resources/
│       └── product/
│           └── product.go          # Your data definition (you wrote this)
├── cmd/
│   └── server/
│       ├── main.go                 # Server entry point (with stubs)
│       ├── product_handlers_generated.go    # HTTP handlers (generated)
│       ├── routes_generated.go              # URL routing (generated)
│       ├── models_generated.go              # Server types (generated)
│       └── openapi_generated.go             # OpenAPI spec (generated)
├── internal/
│   └── storage/
│       └── storage_generated.go             # Storage operations (generated)
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
# Add a new resource type
fabrica add resource Order

# Edit the generated pkg/resources/order/order.go
# Add your OrderSpec and OrderStatus fields

# Regenerate all code
fabrica generate
```

Each resource gets its own complete set of CRUD endpoints automatically.

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

This quick start used **simple mode** which hides Fabrica's advanced features. When you're ready to learn more:

- **[Resource Management Tutorial](./getting-started.md)** (2-4 hours)
  - Learn about labels, annotations, and metadata
  - Understand the Kubernetes-inspired resource model
  - Add search and filtering capabilities

- **[Advanced Patterns Guide](./architecture.md)** (1-2 days)
  - Event-driven architecture
  - Reconciliation loops
  - Multi-version APIs
  - Custom policies

- **[Validation Guide](./validation.md)**
  - Struct tag validation
  - Custom validators
  - Kubernetes-style validation

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
