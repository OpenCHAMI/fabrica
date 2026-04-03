<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Usage Guide

Quick reference for common Fabrica commands and workflows.

## Table of Contents
- [Installation Verification](#installation-verification)
- [Project Initialization](#project-initialization)
- [Resource Management](#resource-management)
- [Code Generation](#code-generation)
- [Running Your API](#running-your-api)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

---

## Installation Verification

**Check if Fabrica is installed:**
```bash
fabrica version
```

**Expected output:**
```
Fabrica v0.3.1
```

If you get "command not found", see [Installation](README.md#-installation).

---

## Project Initialization

### Basic Project (File Storage)
```bash
fabrica init my-api
cd my-api
```

Creates a project with:
- ✅ File-based storage (development-friendly)
- ✅ Single API group (`api.example/v1`)
- ✅ Basic CRUD endpoints
- ✅ OpenAPI documentation

### Production Project (Database Backend)
```bash
fabrica init my-api --storage-type ent --db sqlite
```

Supported databases:
- `sqlite` - Single-file database (good for development/testing)
- `postgres` - Production-grade PostgreSQL
- `mysql` - Production-grade MySQL

### Project with Events & Reconciliation
```bash
fabrica init my-api --events --reconcile --storage-type ent
```

Adds:
- 📡 CloudEvents publishing on resource CRUD
- 🔄 Reconciliation controller framework
- 🗃️ Database storage for durability

### Custom API Group
```bash
fabrica init my-api --group mycompany.io
```

Results in APIs like: `mycompany.io/v1/resources` (v1 is the default storage version)

### View All Options
```bash
fabrica init --help
```

---

## Resource Management

### Add a Resource
```bash
cd my-api
fabrica add resource User
```

Generates:
- `apis/api.example/v1/user_types.go` - Resource struct definition
- Handler stubs in `cmd/server/user_handlers_generated.go`
- Storage functions in `internal/storage/user_storage_generated.go`

### Customize Your Resource
Edit the generated resource file (e.g., `apis/api.example/v1/user_types.go`):

```go
// UserSpec defines the desired state
type UserSpec struct {
    Email     string `json:"email" validate:"required,email"`
    FirstName string `json:"firstName" validate:"required"`
    LastName  string `json:"lastName" validate:"required"`
    Role      string `json:"role" validate:"oneof=admin user guest"`
    Active    bool   `json:"active"`
}

// UserStatus defines the observed state
type UserStatus struct {
    LastLogin  *time.Time `json:"lastLogin,omitempty"`
    LoginCount int        `json:"loginCount"`
    Health     string     `json:"health" validate:"oneof=healthy degraded unhealthy"`
}
```

### Add Multiple Resources
```bash
fabrica add resource User
fabrica add resource Post
fabrica add resource Comment
```

---

## Code Generation

### Generate All Code
```bash
fabrica generate
```

Regenerates:
- ✅ API handlers (CRUD endpoints)
- ✅ Storage functions (file or database)
- ✅ OpenAPI specs and Swagger UI
- ✅ HTTP client library
- ✅ CLI tools

### Generate Specific Artifacts
```bash
fabrica generate --handlers        # Only API handlers
fabrica generate --storage         # Only storage layer
fabrica generate --openapi         # Only OpenAPI spec
fabrica generate --client          # Only HTTP client
```

Combine flags:
```bash
fabrica generate --handlers --storage --openapi
```

### Test Local Fabrica Changes Safely

If you are developing Fabrica itself and want a test project to generate code from your local checkout without adding a `replace` line to that project's `go.mod`, use:

```bash
fabrica generate --fabrica-source /path/to/fabrica
```

Or set it once for your shell session:

```bash
export FABRICA_SOURCE_PATH=/path/to/fabrica
fabrica generate
```

This keeps the generated project's dependency graph pinned to released Fabrica unless you explicitly opt into local codegen for that run.

### After Generation
```bash
go mod tidy        # Download dependencies
```

---

## Running Your API

### Development Mode
```bash
go run ./cmd/server
```

**Output:**
```
Server listening on :8080
View API docs: http://localhost:8080/swagger/
```

### Production Build
```bash
go build -o bin/server ./cmd/server
./bin/server
```

### Test with cURL

**Create a resource:**
```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice",
    "email": "alice@example.com",
    "role": "admin"
  }'
```

**List all resources:**
```bash
curl http://localhost:8080/users
```

**Get specific resource:**
```bash
curl http://localhost:8080/users/alice
```

**Update resource:**
```bash
curl -X PUT http://localhost:8080/users/alice \
  -H "Content-Type: application/json" \
  -d '{"email": "alice.new@example.com", "role": "user"}'
```

**Delete resource:**
```bash
curl -X DELETE http://localhost:8080/users/alice
```

**View OpenAPI spec:**
```bash
open http://localhost:8080/swagger/
```

---

## Common Workflows

### 1. Add API Versioning (Hub/Spoke Pattern)

The root-level `apis.yaml` is created by `fabrica init` and drives version management.

```bash
# Add a new version (copies types from latest version)
fabrica add version v1beta1

# Or copy from a specific version
fabrica add version v1beta1 --from v1alpha1

# apis.yaml is updated automatically; regenerate to create handlers
fabrica generate
```

Now your API supports:
- `infra.example.io/v1` - Storage (hub) version
- `infra.example.io/v1beta1` - New spoke version

For details, see [docs/apis-yaml.md](docs/apis-yaml.md) and [docs/versioning.md](docs/versioning.md).

### 2. Switch from File to Database Storage
```bash
# Update .fabrica.yaml
sed -i 's/file/ent/' .fabrica.yaml

# Set database driver
cat >> .fabrica.yaml << 'EOF'
db_driver: sqlite
EOF

# Regenerate
fabrica generate

# Initialize database schema
go run ./cmd/server init-db  # Creates database tables
```

### 3. Add Request Validation
Validation is automatic via struct tags:

```go
type UserSpec struct {
    Email string `json:"email" validate:"required,email"`           // Required, valid email
    Age   int    `json:"age" validate:"required,min=18,max=120"`    // 18-120
    Phone string `json:"phone" validate:"omitempty,len=10"`         // Optional, exactly 10 chars
    Role  string `json:"role" validate:"oneof=admin user guest"`    // One of these values
}
```

Validation runs automatically on POST/PUT/PATCH. Errors return structured responses:
```json
{
  "error": "validation_failed",
  "details": [
    {
      "field": "email",
      "message": "must be a valid email address"
    }
  ]
}
```

### 4. Add Custom Authentication
Edit `cmd/server/middleware.go`:

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        // Validate token...
        next.ServeHTTP(w, r)
    })
}
```

Register in routes:
```go
// In cmd/server/routes.go
router.Use(AuthMiddleware)
```

### 5. Add CloudEvents Publishing
CloudEvents are automatic if initialized with `--events`:

```bash
# Listen for events
curl http://localhost:8080/events
```

Events published for:
- ✅ Resource created: `io.fabrica.resource.created`
- ✅ Resource updated: `io.fabrica.resource.updated`
- ✅ Resource deleted: `io.fabrica.resource.deleted`

### 6. Enable Reconciliation Controller
```bash
# Initialize with reconciliation
fabrica init my-api --reconcile

# Or add to existing project in .fabrica.yaml
features:
  reconciliation:
    enabled: true
    worker_count: 5
    requeue_delay: 5
```

Implement your reconciliation logic in `pkg/reconcilers/resource_reconciler.go`.

---

## Troubleshooting

### "legacy mode is deprecated"
**Problem:** Project initialized without versioning flags

**Solution:**
```bash
# When adding resources, specify API version
fabrica add resource MyResource

# Or reinitialize project with versioning
fabrica init my-api --group myapi.io --storage-version v1
```

### "undefined: v1" after code generation
**Problem:** Generated code references package that isn't imported

**Solution:**
```bash
go mod tidy
```

If still failing, check `apis.yaml` and `.fabrica.yaml`:
```yaml
groups:
  - name: api.example  # Should match your API group
    storageVersion: v1
```

```yaml
features:
  versioning:
    enabled: true
    strategy: header
```

### "database connection refused"
**Problem:** Ent storage configured but database not running

**Solution:**
```bash
# For SQLite (no setup needed)
# For PostgreSQL
docker run -d -e POSTGRES_PASSWORD=secret postgres:latest

# For MySQL
docker run -d -e MYSQL_ROOT_PASSWORD=secret mysql:latest
```

### "OpenAPI spec missing or invalid"
**Problem:** Generation didn't complete

**Solution:**
```bash
# Remove generated files
rm cmd/server/*_generated.go

# Regenerate explicitly
fabrica generate --openapi

# Verify output
curl http://localhost:8080/api/openapi.json | jq .
```

---

## Next Steps

- 📚 **Learn by Example:** [examples/](examples/) directory has complete working projects
- 🎓 **Deep Dive:** [Architecture Guide](docs/reference/architecture.md) explains design patterns
- 🔧 **Customize:** [Code Generation Reference](docs/reference/codegen.md) for template modifications
- 💾 **Storage:** [Storage Guide](docs/guides/storage.md) for backend configuration

---

## Getting Help

- 📖 [Full Documentation](README.md)
- 💬 [GitHub Discussions](https://github.com/openchami/fabrica/discussions)
- 🐛 [Report Issues](https://github.com/openchami/fabrica/issues)
- 📧 Community channels on OpenCHAMI Slack
