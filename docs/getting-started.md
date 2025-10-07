<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Getting Started with Fabrica

This guide will walk you through creating your first REST API with Fabrica in about 15 minutes.

## Prerequisites

- Go 1.23 or later installed
- Basic familiarity with Go
- A terminal

## Installation

Install the Fabrica CLI:

```bash
go install github.com/alexlovelltroy/fabrica/cmd/fabrica@v0.2.5
```

Verify installation:

```bash
fabrica --version
# Output: fabrica version v0.2.5
```

## Create Your First API

### Step 1: Initialize Project

```bash
fabrica init bookstore
cd bookstore
```

This creates:
```
bookstore/
├── cmd/
│   ├── server/          # API server
│   └── client/          # Client code
├── pkg/
│   └── resources/       # Your resource definitions
├── internal/
│   └── storage/         # Generated storage
├── go.mod
├── Makefile
└── README.md
```

### Step 2: Add Your First Resource

```bash
fabrica add resource Book
```

This creates `pkg/resources/book/book.go`:

```go
package book

import (
    "context"
    "github.com/alexlovelltroy/fabrica/pkg/resource"
    "github.com/alexlovelltroy/fabrica/pkg/validation"
)

// Book represents a Book resource
type Book struct {
    resource.Resource
    Spec   BookSpec   `json:"spec" validate:"required"`
    Status BookStatus `json:"status,omitempty"`
}

// BookSpec defines the desired state of Book
type BookSpec struct {
    Name        string `json:"name" validate:"required,k8sname"`
    Description string `json:"description,omitempty" validate:"max=200"`
    // Add your spec fields here
}

// BookStatus defines the observed state of Book
type BookStatus struct {
    Phase   string `json:"phase,omitempty"`
    Message string `json:"message,omitempty"`
    Ready   bool   `json:"ready"`
    // Add your status fields here
}

// Validate implements custom validation logic
func (r *Book) Validate(ctx context.Context) error {
    // Add custom validation here
    return nil
}

func init() {
    resource.RegisterResourcePrefix("Book", "boo")
}
```

### Step 3: Customize Your Resource

Edit `pkg/resources/book/book.go` and add fields to `BookSpec`:

```go
type BookSpec struct {
    Title       string   `json:"title" validate:"required,min=1,max=200"`
    Author      string   `json:"author" validate:"required,min=1,max=100"`
    ISBN        string   `json:"isbn" validate:"required,isbn"`
    Price       float64  `json:"price" validate:"required,gt=0"`
    InStock     bool     `json:"inStock"`
    Categories  []string `json:"categories,omitempty"`
}
```

### Step 4: Generate Code

```bash
go mod tidy
fabrica generate
```

Output:
```
🔧 Generating code...
📦 Found 1 resource(s): Book
  ├─ Registering Book...
  ├─ Generating handlers...
  ├─ Generating storage...
  ├─ Generating OpenAPI spec...
  ├─ Generating client code...
  └─ Done!

✅ Code generation complete!
```

This generates:
- `cmd/server/handlers_generated.go` - REST handlers
- `internal/storage/storage_generated.go` - Storage operations
- `cmd/server/openapi_generated.go` - OpenAPI spec
- `pkg/client/client_generated.go` - Go client library

### Step 5: Run Your API

```bash
go run cmd/server/main.go
```

Your API is now running at `http://localhost:8080`!

## Using Your API

### Create a Book

```bash
curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Book",
    "metadata": {
      "name": "golang-guide"
    },
    "spec": {
      "title": "The Go Programming Language",
      "author": "Alan Donovan",
      "isbn": "978-0134190440",
      "price": 44.99,
      "inStock": true,
      "categories": ["programming", "go"]
    }
  }'
```

Response:
```json
{
  "apiVersion": "v1",
  "kind": "Book",
  "metadata": {
    "name": "golang-guide",
    "uid": "boo-abc123def456",
    "createdAt": "2025-10-05T10:00:00Z"
  },
  "spec": {
    "title": "The Go Programming Language",
    "author": "Alan Donovan",
    "isbn": "978-0134190440",
    "price": 44.99,
    "inStock": true,
    "categories": ["programming", "go"]
  }
}
```

### List Books

```bash
curl http://localhost:8080/books
```

### Get a Specific Book

```bash
curl http://localhost:8080/books/boo-abc123def456
```

### Update a Book

```bash
curl -X PUT http://localhost:8080/books/boo-abc123def456 \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "v1",
    "kind": "Book",
    "metadata": {
      "name": "golang-guide",
      "uid": "boo-abc123def456"
    },
    "spec": {
      "title": "The Go Programming Language",
      "author": "Alan Donovan",
      "isbn": "978-0134190440",
      "price": 39.99,
      "inStock": false,
      "categories": ["programming", "go"]
    }
  }'
```

### Delete a Book

```bash
curl -X DELETE http://localhost:8080/books/boo-abc123def456
```

## Understanding the Resource Model

Fabrica uses a Kubernetes-style resource model:

```go
type Book struct {
    APIVersion string        // "v1"
    Kind       string        // "Book"
    Metadata   Metadata      // Name, UID, labels, annotations
    Spec       BookSpec      // Desired state (your fields)
    Status     BookStatus    // Observed state (system updates)
}
```

**Key concepts:**
- **Spec** - What you want (your data model)
- **Status** - What the system knows (read-only, system updates)
- **Metadata** - Standard fields (name, UID, timestamps, labels)

## Validation

Fabrica provides three levels of validation:

### 1. Struct Tag Validation

```go
type BookSpec struct {
    Title  string  `json:"title" validate:"required,min=1,max=200"`
    Price  float64 `json:"price" validate:"required,gt=0,lt=1000"`
    ISBN   string  `json:"isbn" validate:"required,isbn"`
}
```

**Common validators:**
- `required` - Field must be present
- `min=N,max=N` - Length constraints
- `gt=N,lt=N` - Numeric comparisons
- `email`, `url`, `isbn` - Format validators
- `k8sname` - Kubernetes-style name (lowercase, alphanumeric, hyphens)

### 2. Kubernetes-Style Validators

```go
type BookSpec struct {
    Title string `json:"title" validate:"required,k8sname"`
    //                                   ^^^^^^^^ Kubernetes name rules
}
```

### 3. Custom Validation

```go
func (r *Book) Validate(ctx context.Context) error {
    // Business logic validation
    if r.Spec.Price > 100 && !r.Spec.InStock {
        return errors.New("expensive books must be in stock")
    }
    return nil
}
```

## Storage Options

### File-Based Storage (Default)

Perfect for development:

```go
backend, err := storage.NewFileBackend("./data")
```

Data stored in `./data/` directory as JSON files.

### Database Storage (Production)

Use Ent for production:

```bash
fabrica init myapp --storage=ent --db=postgres
```

See [Storage Guide](storage.md) for details.

## Next Steps

Now that you have a working API:

1. **Add More Resources** - `fabrica add resource Author`
2. **Add Authorization** - See [Policy Guide](policy-casbin.md)
3. **Add Validation** - See [Validation Guide](validation.md)
4. **Use the Client** - Generated Go client in `pkg/client/`
5. **Add Events** - See [Events Guide](events.md)
6. **Deploy** - Build with `go build cmd/server/main.go`

## Common Tasks

### Add Another Resource

```bash
fabrica add resource Author
# Edit pkg/resources/author/author.go
fabrica generate
```

### Regenerate After Changes

```bash
# After editing resource definitions
fabrica generate
```

### Build for Production

```bash
go build -o bookstore-api cmd/server/main.go
./bookstore-api
```

### Run Tests

```bash
go test ./...
```

## Troubleshooting

### Error: "go: updates to go.mod needed"

**Fix:** Run `go mod tidy` before `fabrica generate`

### Error: "no resources found"

**Fix:** Make sure your resource embeds `resource.Resource`:
```go
type MyResource struct {
    resource.Resource  // ← Must embed this
    Spec MyResourceSpec
}
```

### Error: "failed to read embedded template"

**Fix:** Update fabrica: `go install github.com/alexlovelltroy/fabrica/cmd/fabrica@v0.2.5`

## Further Reading

- [Quick Start](quickstart.md) - 30-minute tutorial
- [Resource Model](resource-model.md) - Deep dive into resources
- [Code Generation](codegen.md) - How generation works
- [Authorization](policy-casbin.md) - RBAC/ABAC setup
- [API Reference](https://pkg.go.dev/github.com/alexlovelltroy/fabrica)

## Get Help

- [GitHub Issues](https://github.com/alexlovelltroy/fabrica/issues)
- [Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
- [Documentation](https://github.com/alexlovelltroy/fabrica/tree/main/docs)
