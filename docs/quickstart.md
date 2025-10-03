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
fabrica version
# Output: fabrica version v0.1.0
```

## Step 1: Initialize Your Project

Create a new project using **simple mode** (this hides advanced features):

```bash
# Create and enter project directory
mkdir myshop
cd myshop

# Initialize with simple mode
fabrica init --mode=simple

# This creates:
# - go.mod with necessary dependencies
# - Basic project structure
# - Simple README
```

You'll see:

```
✓ Created go.mod
✓ Created README.md
✓ Created basic project structure

Your project is ready! Next steps:
  1. fabrica add resource Product
  2. fabrica generate
  3. go run cmd/server/main.go
```

## Step 2: Define Your Data

Create a file `product.go` to define your data structure:

```bash
mkdir -p pkg/resources/product
```

Create `pkg/resources/product/product.go`:

```go
package product

// Product represents an item in your store
type Product struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Price       float64 `json:"price"`
    InStock     bool    `json:"inStock"`
}
```

**That's it!** Just a plain Go struct. No inheritance, no magic fields.

## Step 3: Generate Code

Use the Fabrica CLI to generate all the REST API code:

```bash
fabrica add resource Product
```

This command:
- Finds your `Product` struct
- Generates HTTP handlers (Create, Read, Update, Delete, List)
- Generates in-memory storage
- Generates API routes
- Creates a main.go server file

You'll see:

```
Analyzing product.go...
✓ Found Product struct
✓ Generated handlers (cmd/server/product_handlers.go)
✓ Generated storage (internal/storage/product_storage.go)
✓ Generated routes (cmd/server/routes.go)
✓ Generated main (cmd/server/main.go)

Done! Run with: go run cmd/server/main.go
```

## Step 4: Run Your API

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

## Step 5: Test Your API

Open a new terminal and try the API:

### Create a Product

```bash
curl -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{
    "id": "prod-1",
    "name": "Laptop",
    "description": "15-inch laptop",
    "price": 999.99,
    "inStock": true
  }'
```

Response:

```json
{
  "id": "prod-1",
  "name": "Laptop",
  "description": "15-inch laptop",
  "price": 999.99,
  "inStock": true
}
```

### Get All Products

```bash
curl http://localhost:8080/products
```

Response:

```json
[
  {
    "id": "prod-1",
    "name": "Laptop",
    "description": "15-inch laptop",
    "price": 999.99,
    "inStock": true
  }
]
```

### Get a Specific Product

```bash
curl http://localhost:8080/products/prod-1
```

### Update a Product

```bash
curl -X PUT http://localhost:8080/products/prod-1 \
  -H "Content-Type: application/json" \
  -d '{
    "id": "prod-1",
    "name": "Gaming Laptop",
    "description": "High-performance 15-inch laptop",
    "price": 1299.99,
    "inStock": true
  }'
```

### Delete a Product

```bash
curl -X DELETE http://localhost:8080/products/prod-1
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
├── go.mod                          # Dependencies
├── README.md                       # Project README
├── pkg/
│   └── resources/
│       └── product/
│           └── product.go         # Your data definition (you wrote this)
├── cmd/
│   └── server/
│       ├── main.go                # Server entry point (generated)
│       ├── routes.go              # URL routing (generated)
│       └── product_handlers.go    # HTTP handlers (generated)
└── internal/
    └── storage/
        └── product_storage.go     # In-memory storage (generated)
```

### What Fabrica Generated

1. **HTTP Handlers** (`cmd/server/product_handlers.go`):
   - Functions to handle each REST operation
   - JSON marshaling/unmarshaling
   - Error handling

2. **Storage Layer** (`internal/storage/product_storage.go`):
   - Thread-safe in-memory storage
   - CRUD operations
   - List filtering

3. **Server & Routes** (`cmd/server/main.go`, `routes.go`):
   - HTTP server setup
   - URL routing configuration
   - Middleware setup

4. **All boilerplate code** you'd normally write by hand!

### What You Wrote

Just the `Product` struct! That's 7 lines of code to get a full REST API.

## Next Steps

### Add More Resources

Need users? Orders? Categories?

```bash
# Define your struct in a new file
mkdir -p pkg/resources/order
# Create pkg/resources/order/order.go with your Order struct

# Generate code
fabrica add resource Order
```

Each resource gets its own complete set of CRUD endpoints automatically.

### Add Validation

Want to validate input? Add struct tags:

```go
type Product struct {
    ID          string  `json:"id" validate:"required"`
    Name        string  `json:"name" validate:"required,min=3,max=100"`
    Description string  `json:"description"`
    Price       float64 `json:"price" validate:"required,gt=0"`
    InStock     bool    `json:"inStock"`
}
```

Then regenerate:

```bash
fabrica add resource Product --with-validation
```

Now invalid requests return 400 errors with helpful messages!

### Generate Examples

Want to see example code for different scenarios?

```bash
# Generate a validation example
fabrica example validation --level=beginner

# Generated in examples/validation-beginner/
```

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

## Comparison: With and Without Fabrica

### Without Fabrica (Traditional Approach)

To build the same Product API manually, you'd write:

```go
// ~50 lines: HTTP handlers
func CreateProduct(w http.ResponseWriter, r *http.Request) { /* ... */ }
func GetProduct(w http.ResponseWriter, r *http.Request) { /* ... */ }
func UpdateProduct(w http.ResponseWriter, r *http.Request) { /* ... */ }
func DeleteProduct(w http.ResponseWriter, r *http.Request) { /* ... */ }
func ListProducts(w http.ResponseWriter, r *http.Request) { /* ... */ }

// ~30 lines: Storage layer
type ProductStorage struct { /* ... */ }
func (s *ProductStorage) Create(p Product) error { /* ... */ }
func (s *ProductStorage) Get(id string) (*Product, error) { /* ... */ }
// ... more storage methods

// ~20 lines: Server and routing
func main() {
    http.HandleFunc("/products", handleProducts)
    http.HandleFunc("/products/", handleProduct)
    // ... routing logic
}

// ~15 lines: Error handling utilities
// ~10 lines: JSON helpers
```

**Total: ~125 lines of boilerplate code** for ONE resource.

### With Fabrica

```go
// 7 lines: Your data structure
type Product struct {
    ID          string  `json:"id"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Price       float64 `json:"price"`
    InStock     bool    `json:"inStock"`
}
```

**Total: 7 lines of code** + one CLI command.

Fabrica generates all the boilerplate for you!

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
