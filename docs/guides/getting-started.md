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
go install github.com/openchami/fabrica/cmd/fabrica@latest
```

Verify installation:

```bash
fabrica --version
# Output: fabrica version v0.3.0
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
├── .fabrica.yaml        # Project configuration
├── cmd/
│   └── server/          # API server (main.go with stubs)
├── pkg/
│   └── resources/       # Your resource definitions (empty initially)
├── internal/            # Generated code will go here
├── go.mod
├── go.sum
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
    "github.com/openchami/fabrica/pkg/resource"
)

// Book represents a Book resource
type Book struct {
    resource.Resource `json:",inline"`
    Spec              BookSpec   `json:"spec"`
    Status            BookStatus `json:"status"`
}

// BookSpec defines the desired state of Book
type BookSpec struct {
    Title       string `json:"title" validate:"required,min=1,max=100"`
    Author      string `json:"author" validate:"required,min=1,max=50"`
    Description string `json:"description,omitempty" validate:"max=500"`
    Price       float64 `json:"price" validate:"min=0"`
    InStock     bool   `json:"inStock"`
}

// BookStatus defines the observed state of Book
type BookStatus struct {
    Phase       string `json:"phase,omitempty"`
    Message     string `json:"message,omitempty"`
    Ready       bool   `json:"ready"`
    LastUpdated string `json:"lastUpdated,omitempty"`
}
```

func init() {
    resource.RegisterResourcePrefix("Book", "boo")
}
```

### Step 3: Customize Your Resource

Edit `pkg/resources/book/book.go` and modify the `BookSpec` fields as needed:

```go
type BookSpec struct {
    Title       string `json:"title" validate:"required,min=1,max=200"`
    Author      string `json:"author" validate:"required,min=1,max=100"`
    Description string `json:"description,omitempty" validate:"max=500"`
    Price       float64 `json:"price" validate:"min=0"`
    InStock     bool   `json:"inStock"`
}

type BookStatus struct {
    Phase       string `json:"phase,omitempty"`
    Message     string `json:"message,omitempty"`
    Ready       bool   `json:"ready"`
    LastUpdated string `json:"lastUpdated,omitempty"`
}
```

### Step 4: Generate Code

```bash
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

### Step 5: Update Dependencies

After code generation, update your Go module dependencies:

```bash
go mod tidy
```

This resolves all the new imports that were added by the code generator.

### Step 6: Run Your API

```bash
go run ./cmd/server/
```

Your API is now running at `http://localhost:8080`!

## Using Your API

### Create a Book

```bash
curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{
    "name": "golang-guide",
    "title": "The Go Programming Language",
    "author": "Alan Donovan",
    "description": "A comprehensive guide to Go programming",
    "price": 44.99,
    "inStock": true
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
    "createdAt": "2025-10-15T10:00:00Z",
    "updatedAt": "2025-10-15T10:00:00Z"
  },
  "spec": {
    "title": "The Go Programming Language",
    "author": "Alan Donovan",
    "description": "A comprehensive guide to Go programming",
    "price": 44.99,
    "inStock": true
  },
  "status": {
    "phase": "Active",
    "ready": true,
    "lastUpdated": "2025-10-15T10:00:00Z"
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
    "name": "golang-guide",
    "title": "The Go Programming Language",
    "author": "Alan Donovan",
    "description": "Updated comprehensive guide to Go programming",
    "price": 39.99,
    "inStock": false
  }'
```

### Delete a Book

```bash
curl -X DELETE http://localhost:8080/books/boo-abc123def456
```

## Understanding the Resource Model

Fabrica uses a Kubernetes-style resource model with an envelope pattern:

```go
type Book struct {
    resource.Resource `json:",inline"`  // Embeds apiVersion, kind, metadata
    Spec              BookSpec         `json:"spec"`      // Your desired state
    Status            BookStatus       `json:"status"`    // Observed state
}
```

**Key concepts:**
- **Spec** - What you want (your data model)
- **Status** - What the system observes (runtime state, health info)
- **Metadata** - Standard fields (name, UID, timestamps, labels)
- **Resource** - Embedded base with apiVersion, kind, metadata

## Validation

Fabrica uses struct tag validation for request validation:

```go
type BookSpec struct {
    Title  string  `json:"title" validate:"required,min=1,max=200"`
    Price  float64 `json:"price" validate:"min=0"`
    Author string  `json:"author" validate:"required,min=1,max=100"`
}
```

**Common validators:**
- `required` - Field must be present
- `min=N,max=N` - Length/value constraints
- `gt=N,lt=N` - Numeric comparisons
- `email`, `url`, `ip` - Format validators
- `oneof=a b c` - Enum validation

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
2. **Add Custom Middleware** - Implement authentication/authorization in `internal/middleware/`
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

**Fix:** Run `go mod tidy` after `fabrica generate` to resolve all new imports created by the code generator.

### Error: "no resources found"

**Fix:** Make sure your resource embeds `resource.Resource`:
```go
type MyResource struct {
    resource.Resource  // ← Must embed this
    Spec MyResourceSpec
}
```

### Error: "failed to read embedded template"

**Fix:** Update fabrica: `go install github.com/openchami/fabrica/cmd/fabrica@latest`

## Further Reading

- [Quick Start](quickstart.md) - 30-minute tutorial
- [Resource Model](resource-model.md) - Deep dive into resources
- [Code Generation](codegen.md) - How generation works
- [Validation](validation.md) - Request validation
- [Events](events.md) - Event-driven architecture
- [API Reference](https://pkg.go.dev/github.com/openchami/fabrica)

## Get Help

- [GitHub Issues](https://github.com/openchami/fabrica/issues)
- [Discussions](https://github.com/openchami/fabrica/discussions)
- [Documentation](https://github.com/openchami/fabrica/tree/main/docs)
