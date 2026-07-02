<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 12: Storage Annotations

**Time: 25 minutes** | **Difficulty: Intermediate** | **Prerequisites: Example 1, 3**

Learn how to use **storage annotations** to customize how Fabrica generates database schemas and storage adapters for your resources.

## What You'll Learn

- ✅ Use `+fabrica:storage=dedicated` to generate per-resource database tables
- ✅ Apply `+fabrica:field:storage=hashed:bcrypt` for secure password/token storage
- ✅ Mark fields as `sensitive` to exclude them from logs
- ✅ Make fields `immutable` to prevent updates after creation
- ✅ Add database `index` and `unique` constraints
- ✅ Set `default` values for optional fields
- ✅ Understand when to use dedicated vs generic storage

## What You'll Build

A **User Management API** with proper security practices:
- Passwords automatically bcrypt-hashed (never stored in plaintext)
- Email addresses indexed and unique (fast lookups, no duplicates)
- Usernames immutable (can't change after creation)
- API keys marked sensitive (excluded from debug logs)
- Account status with default values
- Per-user database table with type-safe queries

## The Problem

By default, Fabrica stores all resources in a single generic `resource` table with JSON encoding. This works well for simple APIs, but has limitations:

❌ **No type-safe queries** - Can't efficiently query specific fields
❌ **No database constraints** - Can't enforce uniqueness or indexes at DB level
❌ **No secure hashing** - Passwords/tokens stored as plaintext JSON
❌ **No immutability** - Any field can be changed after creation

## The Solution: Storage Annotations

Annotate your resource types with `+fabrica:` directives to generate:

✅ **Dedicated database tables** - One table per resource type
✅ **Type-safe schemas** - Strongly-typed columns with proper constraints
✅ **Automatic bcrypt hashing** - Secure password/token storage
✅ **Field-level controls** - Sensitive, immutable, indexed, unique, defaults
✅ **Custom storage adapters** - Envelope↔Entity mapping with security rules

## Project Setup

### Step 1: Initialize Project

```bash
cd /tmp
fabrica init user-service --storage-type=ent
cd user-service
```

**What this creates:**
```
user-service/
├── cmd/server/main.go      # Server entry point (storage/routes commented out)
├── apis.yaml               # API group configuration
└── apis/
    └── example.fabrica.dev/
        └── v1/             # Your API version directory (empty)
```

### Step 2: Add User Resource

```bash
fabrica add resource User
```

**What this creates:**
```
apis/example.fabrica.dev/v1/user_types.go    # User resource definition template
```

### Step 3: Add Storage Annotations

Edit `apis/example.fabrica.dev/v1/user_types.go`:

```go
package v1

import (
	metav1 "github.com/openchami/fabrica/pkg/meta/v1"
)

// User represents a system user with authentication credentials.
//
// +fabrica:resource
// +fabrica:storage=dedicated
type User struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   metav1.Metadata `json:"metadata"`
	Spec       UserSpec       `json:"spec"`
	Status     UserStatus     `json:"status,omitempty"`
}

type UserSpec struct {
	// Username is the unique identifier for this user.
	// Immutable after creation - cannot be changed.
	//
	// +fabrica:field:immutable
	// +fabrica:field:unique
	// +fabrica:field:index
	Username string `json:"username" validate:"required,min=3,max=32,alphanum"`

	// Email address for the user.
	// Must be unique across all users.
	//
	// +fabrica:field:unique
	// +fabrica:field:index
	Email string `json:"email" validate:"required,email"`

	// Password is the user's password (plaintext in API request).
	// Automatically hashed with bcrypt before storage.
	// Never returned in API responses.
	//
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Password string `json:"password" validate:"required,min=8"`

	// FullName is the user's display name.
	FullName string `json:"fullName" validate:"required,min=1,max=128"`

	// Role defines the user's permission level.
	//
	// +fabrica:field:default=user
	// +fabrica:field:index
	Role string `json:"role" validate:"required,oneof=admin user readonly"`

	// APIKey is an optional API key for programmatic access.
	// Automatically hashed with bcrypt if provided.
	// Never returned in API responses.
	//
	// +fabrica:field:storage=hashed:bcrypt:cost=10
	// +fabrica:field:sensitive
	APIKey string `json:"apiKey,omitempty" validate:"omitempty,min=32,max=128"`

	// Active indicates whether the user account is enabled.
	//
	// +fabrica:field:default=true
	Active bool `json:"active"`
}

type UserStatus struct {
	// Conditions track the user account lifecycle.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastLogin tracks when the user last authenticated.
	LastLogin *metav1.Time `json:"lastLogin,omitempty"`

	// LoginCount tracks total successful logins.
	LoginCount int `json:"loginCount,omitempty"`
}

// Validate performs custom validation for User resources.
func (u *User) Validate() error {
	return nil  // Could add custom logic here
}
```

**Key Annotations Explained:**

| Annotation | Purpose | Example Use |
|------------|---------|-------------|
| `+fabrica:storage=dedicated` | Generate dedicated table | User → `users` table |
| `+fabrica:field:storage=hashed:bcrypt:cost=12` | Bcrypt hash before storage | Passwords, API keys |
| `+fabrica:field:sensitive` | Exclude from logs/debug | Credentials, tokens |
| `+fabrica:field:immutable` | Prevent updates | Username, password |
| `+fabrica:field:unique` | Database unique constraint | Email, username |
| `+fabrica:field:index` | Database index for fast queries | Email, username, role |
| `+fabrica:field:default=value` | Default value if omitted | `role=user`, `active=true` |

### Step 4: Generate Code

```bash
fabrica generate
go mod tidy
```

**What this generates:**

```
Generated files:
├── internal/storage/ent/schema/
│   ├── user.go                    # Dedicated Ent schema for User (NEW!)
│   ├── resource.go                # Generic resource table (existing)
│   ├── label.go                   # Label table (existing)
│   └── annotation.go              # Annotation table (existing)
│
├── internal/storage/
│   └── ent_adapter_user.go        # User-specific storage adapter (NEW!)
│
├── internal/handlers/
│   └── user_handlers_generated.go # CRUD handlers
│
└── api/openapi.yaml               # OpenAPI spec
```

**Key Generated Files:**

#### `internal/storage/ent/schema/user.go`
```go
// Generated by Fabrica
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"golang.org/x/crypto/bcrypt"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("uid").Unique().Immutable(),
		field.String("api_version"),
		field.String("kind"),
		field.String("name").Unique().Immutable(),
		field.String("namespace"),

		// Spec fields with annotations applied
		field.String("username").Unique().Immutable(),
		field.String("email").Unique(),
		field.String("password").Sensitive().Immutable(),  // bcrypt hashed
		field.String("full_name"),
		field.String("role").Default("user"),
		field.String("api_key").Optional().Sensitive(),    // bcrypt hashed
		field.Bool("active").Default(true),

		// Status fields
		field.Time("last_login").Optional().Nillable(),
		field.Int("login_count").Default(0),

		field.Time("created_at"),
		field.Time("updated_at"),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("username"),
		index.Fields("email"),
		index.Fields("role"),
	}
}

func (User) Hooks() []ent.Hook {
	return []ent.Hook{
		// Bcrypt hook for password field
		func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				if um, ok := m.(*ent.UserMutation); ok {
					if password, exists := um.Password(); exists && password != "" {
						hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
						if err != nil {
							return nil, fmt.Errorf("hash password: %w", err)
						}
						um.SetPassword(string(hashed))
					}
				}
				return next.Mutate(ctx, m)
			})
		},
		// Bcrypt hook for api_key field (if provided)
		// ... similar pattern ...
	}
}
```

#### `internal/storage/ent_adapter_user.go`
```go
// Generated by Fabrica
package storage

import (
	"context"
	"fmt"
	v1 "github.com/test/user-service/apis/example.fabrica.dev/v1"
	"github.com/test/user-service/internal/storage/ent"
)

// ToEntUser converts a User envelope to an Ent UserCreate builder.
// Respects immutability constraints.
func ToEntUser(ctx context.Context, client *ent.Client, user *v1.User) (*ent.UserCreate, error) {
	builder := client.User.Create()

	builder.SetUID(user.Metadata.UID)
	builder.SetAPIVersion(user.APIVersion)
	builder.SetKind(user.Kind)
	builder.SetName(user.Metadata.Name)
	builder.SetNamespace(user.Metadata.Namespace)

	// Spec fields (plaintext - bcrypt happens in hooks)
	builder.SetUsername(user.Spec.Username)
	builder.SetEmail(user.Spec.Email)
	builder.SetPassword(user.Spec.Password)  // Will be hashed by hook
	builder.SetFullName(user.Spec.FullName)
	builder.SetRole(user.Spec.Role)
	if user.Spec.APIKey != "" {
		builder.SetAPIKey(user.Spec.APIKey)  // Will be hashed by hook
	}
	builder.SetActive(user.Spec.Active)

	return builder, nil
}

// FromEntUser converts an Ent User entity to a User envelope.
// Sensitive fields (password, apiKey) are EXCLUDED from response.
func FromEntUser(ctx context.Context, entity *ent.User) (*v1.User, error) {
	user := &v1.User{
		APIVersion: entity.APIVersion,
		Kind:       entity.Kind,
		Metadata: metav1.Metadata{
			UID:       entity.UID,
			Name:      entity.Name,
			Namespace: entity.Namespace,
			CreatedAt: metav1.Time{Time: entity.CreatedAt},
			UpdatedAt: metav1.Time{Time: entity.UpdatedAt},
		},
		Spec: v1.UserSpec{
			Username: entity.Username,
			Email:    entity.Email,
			// Password:  EXCLUDED (sensitive)
			FullName:  entity.FullName,
			Role:      entity.Role,
			// APIKey:   EXCLUDED (sensitive)
			Active:    entity.Active,
		},
		Status: v1.UserStatus{
			LoginCount: entity.LoginCount,
		},
	}

	if entity.LastLogin != nil {
		user.Status.LastLogin = &metav1.Time{Time: *entity.LastLogin}
	}

	return user, nil
}

// UpdateEntUser updates an existing Ent User.
// Respects immutability (username, password cannot be changed).
func UpdateEntUser(ctx context.Context, entity *ent.User, updated *v1.User) *ent.UserUpdateOne {
	builder := entity.Update()

	// Immutable fields are NOT updated (username, password)

	// Mutable fields
	builder.SetEmail(updated.Spec.Email)
	builder.SetFullName(updated.Spec.FullName)
	builder.SetRole(updated.Spec.Role)
	builder.SetActive(updated.Spec.Active)

	return builder
}
```

### Step 5: Run Database Migrations

Since we're using Ent with a dedicated schema, we need to run migrations:

```bash
# Generate Ent code
go generate ./internal/storage/ent

# Create database and run migrations
go run ./cmd/server migrate
```

This creates the SQLite database with the User table.

### Step 6: Enable Storage and Routes

Edit `cmd/server/main.go` and uncomment the storage initialization and route registration sections.

### Step 7: Run the Server

```bash
go run ./cmd/server
```

Server starts on `http://localhost:8080`.

## Testing the API

### 1. Create User (Password Hashed Automatically)

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "User",
    "metadata": {
      "name": "alice"
    },
    "spec": {
      "username": "alice",
      "email": "alice@example.com",
      "password": "MySecurePassword123!",
      "fullName": "Alice Smith",
      "role": "admin"
    }
  }'
```

**Response:**
```json
{
  "apiVersion": "example.fabrica.dev/v1",
  "kind": "User",
  "metadata": {
    "uid": "550e8400-e29b-41d4-a716-446655440000",
    "name": "alice",
    "createdAt": "2025-06-30T12:00:00Z",
    "updatedAt": "2025-06-30T12:00:00Z"
  },
  "spec": {
    "username": "alice",
    "email": "alice@example.com",
    "fullName": "Alice Smith",
    "role": "admin",
    "active": true
  },
  "status": {
    "loginCount": 0
  }
}
```

**Notice:**
- ✅ `password` is **NOT in the response** (sensitive field)
- ✅ `active` defaulted to `true` (default annotation)
- ✅ `role` is `admin` (we provided it)
- ✅ Password is bcrypt-hashed in database (never stored plaintext)

### 2. Verify Password is Hashed in Database

```bash
sqlite3 user-service.db "SELECT username, password FROM users WHERE username='alice';"
```

**Output:**
```
alice|$2a$12$rQ7h9p.kH5zT8W2xJ3Y9KOjXvZ6Y8mN4pL2qR5sT6uV7wX8yZ9aBC
```

✅ Bcrypt hash - password is secure!

### 3. Try Duplicate Username (Unique Constraint)

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "User",
    "metadata": {
      "name": "alice2"
    },
    "spec": {
      "username": "alice",
      "email": "alice2@example.com",
      "password": "AnotherPassword456!",
      "fullName": "Alice Jones",
      "role": "user"
    }
  }'
```

**Response:**
```json
{
  "error": "username already exists: unique constraint violation"
}
```

✅ Database enforces uniqueness!

### 4. Try Duplicate Email (Unique Constraint)

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "User",
    "metadata": {
      "name": "bob"
    },
    "spec": {
      "username": "bob",
      "email": "alice@example.com",
      "password": "BobPassword789!",
      "fullName": "Bob Johnson",
      "role": "user"
    }
  }'
```

**Response:**
```json
{
  "error": "email already exists: unique constraint violation"
}
```

✅ Email uniqueness enforced!

### 5. Create User with Default Role

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "User",
    "metadata": {
      "name": "charlie"
    },
    "spec": {
      "username": "charlie",
      "email": "charlie@example.com",
      "password": "CharliePass123!",
      "fullName": "Charlie Brown"
    }
  }'
```

**Response:**
```json
{
  "spec": {
    "username": "charlie",
    "email": "charlie@example.com",
    "fullName": "Charlie Brown",
    "role": "user",
    "active": true
  }
}
```

✅ `role` defaulted to `user` (default annotation)!

### 6. Try to Change Username (Immutable Field)

```bash
# Get Charlie's UID first
CHARLIE_UID=$(curl -s http://localhost:8080/api/v1/users?name=charlie | jq -r '.items[0].metadata.uid')

# Try to update username
curl -X PUT "http://localhost:8080/api/v1/users/${CHARLIE_UID}" \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "User",
    "metadata": {
      "uid": "'${CHARLIE_UID}'",
      "name": "charlie"
    },
    "spec": {
      "username": "charlie_new",
      "email": "charlie@example.com",
      "password": "CharliePass123!",
      "fullName": "Charlie Brown Updated",
      "role": "user",
      "active": true
    }
  }'
```

**Result:**
```json
{
  "spec": {
    "username": "charlie",
    "email": "charlie@example.com",
    "fullName": "Charlie Brown Updated"
  }
}
```

✅ Username **NOT changed** (immutable)!
✅ FullName **WAS changed** (mutable)!

### 7. List Users with Fast Index Queries

```bash
# List by role (indexed field - fast query)
curl "http://localhost:8080/api/v1/users?label=role=admin"

# List by username (indexed field - fast query)
curl "http://localhost:8080/api/v1/users?name=alice"
```

✅ Database indexes make these queries fast!

## What Happened Behind the Scenes

### 1. Annotation Parsing
Fabrica parsed your `+fabrica:` comments and stored them in `ResourceMetadata`:

```go
metadata := ResourceMetadata{
    Name: "User",
    StorageMode: "dedicated",  // from +fabrica:storage=dedicated
    Annotations: map[string]annotations.Annotation{
        "Username": {
            Field: "Username",
            Immutable: true,
            Unique: true,
            Index: true,
        },
        "Password": {
            Field: "Password",
            Storage: "hashed",
            HashAlgorithm: "bcrypt",
            BcryptCost: 12,
            Sensitive: true,
            Immutable: true,
        },
        // ... etc
    },
}
```

### 2. Dedicated Schema Generation
Fabrica generated `internal/storage/ent/schema/user.go` with:

- **Typed columns** for each spec field
- **Bcrypt hooks** for password/apiKey hashing
- **Database indexes** on username, email, role
- **Unique constraints** on username, email
- **Immutable constraints** on username, password
- **Default values** for role, active

### 3. Storage Adapter Generation
Fabrica generated `internal/storage/ent_adapter_user.go` with:

- **ToEntUser()** - Converts API envelope → Ent entity (plaintext passwords - hooks hash them)
- **FromEntUser()** - Converts Ent entity → API envelope (EXCLUDES sensitive fields)
- **UpdateEntUser()** - Updates mutable fields only (respects immutable constraints)

### 4. Handler Integration
The generated handlers in `internal/handlers/user_handlers_generated.go` automatically:

- Call `ToEntUser()` on create
- Call `FromEntUser()` on read/list (sensitive fields excluded)
- Call `UpdateEntUser()` on update (immutable fields protected)

## Annotation Reference

### Resource-Level Annotations

```go
// +fabrica:resource
// +fabrica:storage=dedicated
type User struct { ... }
```

| Annotation | Values | Description |
|------------|--------|-------------|
| `+fabrica:resource` | (none) | Marks this type as a Fabrica resource |
| `+fabrica:storage` | `generic` (default), `dedicated` | Storage mode |

### Field-Level Annotations

```go
// +fabrica:field:storage=hashed:bcrypt:cost=12
// +fabrica:field:sensitive
// +fabrica:field:immutable
// +fabrica:field:unique
// +fabrica:field:index
// +fabrica:field:default=value
Password string `json:"password"`
```

| Annotation | Values | Description |
|------------|--------|-------------|
| `storage` | `standard` (default), `hashed` | How to store the field |
| `hashed` sub-options | `bcrypt`, `cost=N` | Bcrypt algorithm with cost (4-31) |
| `sensitive` | (flag) | Exclude from logs, API responses |
| `immutable` | (flag) | Prevent updates after creation |
| `unique` | (flag) | Database unique constraint |
| `index` | (flag) | Database index for fast queries |
| `default` | `value` | Default value if field omitted |

### Validation

Annotations are validated at generation time:

```bash
fabrica generate
```

**Validation Rules:**
- ✅ Bcrypt cost must be 4-31 (default: 12)
- ✅ `hashed` requires string field type
- ✅ `default` value must match field type
- ✅ `immutable` + `unique` on same field is allowed
- ✅ Database compatibility checked (SQLite, PostgreSQL, MySQL)

**Validation Errors:**
```
Error: invalid bcrypt cost 50 for field Password (must be 4-31)
Error: cannot hash non-string field Age
Error: default value "abc" invalid for boolean field Active
```

## When to Use Dedicated Storage

### Use Dedicated Storage When:

✅ **Security requirements** - Need bcrypt hashing, sensitive field handling
✅ **Performance requirements** - Need database indexes for fast queries
✅ **Data integrity** - Need unique constraints, immutability
✅ **Complex queries** - Need to query specific fields efficiently
✅ **High volume** - Resource has many instances (users, devices, events)

### Use Generic Storage When:

✅ **Simple CRUD** - Basic create/read/update/delete is sufficient
✅ **Low volume** - Few resource instances
✅ **Rapid prototyping** - Want to iterate quickly without schema changes
✅ **Flexible schema** - Schema changes frequently

## Security Best Practices

### 1. Always Hash Passwords

```go
// ✅ CORRECT
// +fabrica:field:storage=hashed:bcrypt:cost=12
// +fabrica:field:sensitive
// +fabrica:field:immutable
Password string `json:"password"`

// ❌ WRONG - plaintext password
Password string `json:"password"`
```

### 2. Mark Sensitive Fields

```go
// ✅ CORRECT - excluded from responses
// +fabrica:field:sensitive
APIKey string `json:"apiKey,omitempty"`

// ❌ WRONG - API key returned in responses
APIKey string `json:"apiKey,omitempty"`
```

### 3. Make Credentials Immutable

```go
// ✅ CORRECT - can't change password after creation
// +fabrica:field:immutable
Password string `json:"password"`

// ❌ WRONG - password can be changed
Password string `json:"password"`
```

### 4. Enforce Uniqueness

```go
// ✅ CORRECT - database enforces uniqueness
// +fabrica:field:unique
// +fabrica:field:index
Email string `json:"email"`

// ❌ WRONG - application must check uniqueness
Email string `json:"email"`
```

### 5. Use Strong Bcrypt Cost

```go
// ✅ CORRECT - OWASP recommended (cost 12+)
// +fabrica:field:storage=hashed:bcrypt:cost=12
Password string `json:"password"`

// ⚠️  WEAK - too fast (cost <10)
// +fabrica:field:storage=hashed:bcrypt:cost=6
Password string `json:"password"`
```

## Advanced Patterns

### Multi-Factor Authentication

```go
type UserSpec struct {
	// Primary password
	// +fabrica:field:storage=hashed:bcrypt:cost=12
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	Password string `json:"password"`

	// TOTP secret for 2FA
	// +fabrica:field:sensitive
	// +fabrica:field:immutable
	TOTPSecret string `json:"totpSecret,omitempty"`

	// Backup codes (hashed)
	// +fabrica:field:storage=hashed:bcrypt:cost=10
	// +fabrica:field:sensitive
	BackupCodes []string `json:"backupCodes,omitempty"`
}
```

### API Key Rotation

```go
type UserSpec struct {
	// Current API key
	// +fabrica:field:storage=hashed:bcrypt:cost=10
	// +fabrica:field:sensitive
	APIKey string `json:"apiKey,omitempty"`

	// Previous API key (for rotation grace period)
	// +fabrica:field:storage=hashed:bcrypt:cost=10
	// +fabrica:field:sensitive
	PreviousAPIKey string `json:"previousAPIKey,omitempty"`
}
```

### Soft Delete with Immutable Username

```go
type UserSpec struct {
	// Username can't be changed
	// +fabrica:field:immutable
	// +fabrica:field:unique
	// +fabrica:field:index
	Username string `json:"username"`

	// Soft delete flag (mutable)
	// +fabrica:field:default=false
	// +fabrica:field:index
	Deleted bool `json:"deleted"`
}
```

## Troubleshooting

### Error: "bcrypt cost must be 4-31"

```bash
Error: invalid bcrypt cost 50 for field Password (must be 4-31)
```

**Fix:** Use cost between 4-31. OWASP recommends 12+:

```go
// +fabrica:field:storage=hashed:bcrypt:cost=12
Password string `json:"password"`
```

### Error: "cannot hash non-string field"

```bash
Error: cannot hash non-string field Age
```

**Fix:** Only string fields can be hashed:

```go
// ✅ CORRECT
// +fabrica:field:storage=hashed:bcrypt
APIKey string `json:"apiKey"`

// ❌ WRONG - Age is int
// +fabrica:field:storage=hashed:bcrypt
Age int `json:"age"`
```

### Error: "unique constraint violation"

```bash
{
  "error": "username already exists: unique constraint violation"
}
```

**Fix:** This is correct behavior! The database is enforcing uniqueness. Change the username to a unique value.

### Sensitive Fields Appearing in Logs

If sensitive fields appear in debug logs:

1. Check annotations are correct:
   ```go
   // +fabrica:field:sensitive
   Password string `json:"password"`
   ```

2. Regenerate code:
   ```bash
   fabrica generate
   go mod tidy
   ```

3. Restart server:
   ```bash
   go run ./cmd/server
   ```

### Password Not Hashed

If passwords appear in plaintext in database:

1. Check annotation format:
   ```go
   // ✅ CORRECT
   // +fabrica:field:storage=hashed:bcrypt:cost=12

   // ❌ WRONG (typo)
   // +fabrica:field:storage=hash:bcrypt:cost=12
   ```

2. Verify Ent hooks were generated:
   ```bash
   grep -A 10 "func.*Hook" internal/storage/ent/schema/user.go
   ```

3. Regenerate Ent code:
   ```bash
   go generate ./internal/storage/ent
   ```

## Next Steps

1. **Add Authentication** - Implement login endpoint with password verification
2. **Add Authorization** - Use JWT tokens for protected endpoints
3. **Add Password Reset** - Implement secure password reset flow
4. **Add Audit Logging** - Track user actions with CloudEvents (Example 5)
5. **Add Rate Limiting** - Prevent brute-force attacks

## Key Takeaways

✅ Storage annotations customize database schemas per-resource
✅ Bcrypt hashing protects passwords/tokens (never plaintext)
✅ Sensitive fields are excluded from API responses
✅ Immutable fields prevent updates after creation
✅ Unique constraints prevent duplicates at database level
✅ Indexes speed up queries on common fields
✅ Default values simplify API requests
✅ Dedicated storage enables type-safe queries

## Related Examples

- **Example 1** - Basic CRUD (prerequisite)
- **Example 3** - FRU Service with Ent storage (prerequisite)
- **Example 5** - CloudEvents for user activity tracking
- **Example 9** - Advanced Ent queries for user management

## Resources

- [Fabrica Documentation](../../docs/)
- [Ent Framework](https://entgo.io/)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [Bcrypt Wikipedia](https://en.wikipedia.org/wiki/Bcrypt)

---

**Questions?** Open an issue at https://github.com/openchami/fabrica/issues

Happy building! 🔐
