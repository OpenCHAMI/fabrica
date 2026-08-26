# Fabrica Annotations Package

**Package:** `github.com/openchami/fabrica/pkg/annotations`
**Status:** Phase 1 Complete
**Version:** 1.0.0

## Overview

The annotations package provides parsing and validation for Fabrica's `+fabrica:` code generation directives. These annotations allow developers to control how resources are stored in the database, including dedicated table schemas, field-level hashing, encryption, indexes, and constraints.

## Table of Contents

- [Quick Start](#quick-start)
- [Annotation Reference](#annotation-reference)
- [API Documentation](#api-documentation)
- [Examples](#examples)
- [Integration Guide](#integration-guide)
- [Validation Rules](#validation-rules)
- [Database Compatibility](#database-compatibility)
- [Error Handling](#error-handling)
- [Future Enhancements](#future-enhancements-phase-5)

---

## Quick Start

### Basic Usage

```go
import (
    "go/ast"
    "go/parser"
    "go/token"
    "github.com/openchami/fabrica/pkg/annotations"
)

// Parse Go source file
fset := token.NewFileSet()
file, _ := parser.ParseFile(fset, "token_types.go", src, parser.ParseComments)

// Find resource type and parse annotations
var typeSpec *ast.TypeSpec
var docComments *ast.CommentGroup

ast.Inspect(file, func(n ast.Node) bool {
    if gd, ok := n.(*ast.GenDecl); ok {
        for _, spec := range gd.Specs {
            if ts, ok := spec.(*ast.TypeSpec); ok {
                typeSpec = ts
                docComments = gd.Doc
                return false
            }
        }
    }
    return true
})

// Parse annotations
annots, err := annotations.ParseResourceAnnotations(typeSpec, docComments)
if err != nil {
    log.Fatalf("Parse error: %v", err)
}

// Validate
if err := annotations.Validate(annots); err != nil {
    log.Fatalf("Validation error: %v", err)
}

// Use annotations
if annots.StorageMode == annotations.StorageModeDedicated {
    fmt.Println("Using dedicated table storage")
}
```

### Example Resource Definition

```go
package v1

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TokenSpec   `json:"spec,omitempty"`
    Status TokenStatus `json:"status,omitempty"`
}

type TokenSpec struct {
    // +fabrica:field:storage=hashed:bcrypt:cost=12
    // +fabrica:field:sensitive
    // +fabrica:field:immutable
    Value string `json:"value"`

    // +fabrica:field:index
    // +fabrica:field:unique
    Name string `json:"name"`

    // +fabrica:field:default=false
    Revoked bool `json:"revoked"`
}
```

---

## Annotation Reference

### Resource-Level Annotations

#### `+fabrica:resource`

Marks a type as a Fabrica resource. Required for code generation.

**Example:**
```go
// +fabrica:resource
type Token struct { ... }
```

---

#### `+fabrica:storage=<mode>`

Defines how the resource is persisted in the database.

**Values:**
- `generic` (default) - All resources stored in one `resources` table with JSON `spec`/`status` columns
- `dedicated` - Resource gets its own table with flattened field columns

**Example:**
```go
// +fabrica:storage=dedicated
type Token struct { ... }
```

**Validation:**
- Dedicated storage requires at least one field annotation

---

#### `+fabrica:index:fields=<f1,f2>[:name=<id>][:unique][:type=<type>]`

Declares a **multi-column** index. Single-column indexes belong on the field
itself (`+fabrica:field:index`); a composite index covering one column is
rejected with that guidance.

Field names are the **Go field names** from the resource's `Spec` struct, in
index order. The generator resolves them to column names.

**Parameters:**

| Parameter | Required | Default | Meaning |
|-----------|----------|---------|---------|
| `fields=` | yes | — | Comma-separated Go field names, in index order |
| `name=` | no | derived by Ent | Explicit index name carried for later schema emission |
| `unique` | no | false | Make the index unique |
| `type=` | no | `btree` | `btree`, `gin`, `gist`, or `hash` |

**Example:**
```go
// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:index:fields=Owner,CreatedAt:name=idx_owner_created
// +fabrica:index:fields=Owner,Slug:unique
type Token struct { ... }
```

**Validation:**
- Requires `+fabrica:storage=dedicated`
- At least two fields
- Every field must name an existing exported Go `Spec` field
- No repeated field within one index
- Explicit names must be portable identifiers and unique on the same resource
- Index type must be supported by the target database (see [Database Compatibility](#database-compatibility))

Repeat the annotation to declare more than one composite index.

---

#### `+fabrica:migration=<policy>`

Constrains what a generated migration may do to this resource's table.

**Values:**
- `unrestricted` (default) - Any migration is permitted
- `additive-only` - Only additive changes (new columns, new indexes); column drops and narrowing type changes must be rejected

**Example:**
```go
// +fabrica:migration=additive-only
type Token struct { ... }
```

**Status:** the policy is parsed and validated as declared intent. Emitting it
into generated schemas and enforcing it in migration tooling belong to follow-up
changes.

---

### Field-Level Annotations

All field annotations start with `+fabrica:field:`.

> **Dedicated storage only.** `nullable`, `notnull`, `size` and `relation`
> require `+fabrica:storage=dedicated` and are rejected during validation
> otherwise. Generic storage keeps spec and status in a single JSON column, so
> per-column intent has nowhere to land.

#### `+fabrica:field:nullable` / `+fabrica:field:notnull`

Declare column nullability intent for dedicated storage. Generator support for
overriding `validate:"required"` inference lands in a follow-up change.

```go
// +fabrica:field:nullable
Note string `json:"note"`

// +fabrica:field:notnull
Owner string `json:"owner"`
```

**Validation:**
- A field may not declare both
- `notnull` conflicts with a relation using `on-delete=set-null`
- `on-delete=set-null` requires an explicit `nullable`

---

#### `+fabrica:field:size=<n>`

Cap the stored width of a string column. Ent `MaxLen(n)` emission lands in a
follow-up change.

```go
// +fabrica:field:size=253
Owner string `json:"owner"`
```

**Validation:**
- `n` must be 1-65535
- the annotated Go field must be `string` or `*string`

**Note:** has no effect on `storage=hashed` fields, whose column width is
already fixed by the hash algorithm's `SchemaType`.

---

#### `+fabrica:field:relation=<kind>:<Target>[:on-delete=<action>]`

Declare a foreign-key relation to another resource type.

**Kinds:** `belongs-to` (many-to-one), `has-many` (one-to-many)

**On-delete actions:** `restrict` (default), `cascade`, `set-null`

```go
// +fabrica:field:relation=belongs-to:User:on-delete=cascade
OwnerID string `json:"owner_id"`
```

**Validation:**
- Target must be a valid Go type name
- `on-delete=set-null` conflicts with `notnull` and with `immutable`

> **Status: parsed and validated, not yet emitted.** Ent edges live in an
> `Edges()` method that requires resolving the target resource's schema, which
> the per-resource template cannot see. Declaring a relation today records
> intent and is checked for consistency, but does **not** change the generated
> schema. Emission is a follow-up change.

---

#### `+fabrica:field:storage=hashed:<algorithm>[:<params>]`

Store a hashed version of the field value instead of plaintext.

**Algorithms:**

##### `bcrypt`
```go
// +fabrica:field:storage=hashed:bcrypt
// +fabrica:field:storage=hashed:bcrypt:cost=12
Value string
```

**Parameters:**
- `cost=N` - Bcrypt cost factor (4-31, default: 12)

**Validation:**
- Cost must be 4-31
- Field must be string type (enforced at generation time)

##### `argon2`
```go
// +fabrica:field:storage=hashed:argon2
// +fabrica:field:storage=hashed:argon2:memory=65536
Password string
```

**Parameters:**
- `memory=N` - Memory cost in KB (1024-1048576, default: 65536)

**Validation:**
- Memory must be 1024-1048576 KB
- Field must be string type

##### `sha256`
```go
// +fabrica:field:storage=hashed:sha256
APIKey string
```

**Parameters:** None

---

#### `+fabrica:field:storage=encrypted:<algorithm>[:<params>]`

Store an encrypted version of the field value.

**Algorithms:**
- `aes128`, `aes192`, `aes256`

**Parameters:**
- `key=<source>` - Key source: `env` (default), `vault`, `kms`

**Example:**
```go
// +fabrica:field:storage=encrypted:aes256:key=vault
SSN string
```

**Validation:**
- Algorithm must be aes128/192/256
- Key source must be env/vault/kms
- Field must be string or []byte

---

#### `+fabrica:field:sensitive`

Marks field as sensitive. Will be excluded from logs and debug output.

**Example:**
```go
// +fabrica:field:sensitive
Password string
```

---

#### `+fabrica:field:immutable`

Prevents field updates after resource creation. Updates will return HTTP 422.

**Example:**
```go
// +fabrica:field:immutable
CreatedBy string
```

**Validation:**
- Cannot combine with `default` (conflicting behaviors)

---

#### `+fabrica:field:unique`

Adds a unique constraint on the field. Duplicate values will fail with HTTP 409.

**Example:**
```go
// +fabrica:field:unique
Email string
```

---

#### `+fabrica:field:index[=<type>]`

Creates a database index on the field for faster queries.

**Index Types:**
- `btree` (default) - Standard B-tree index
- `gin` - PostgreSQL GIN index (full-text, JSON)
- `gist` - PostgreSQL GiST index (spatial)
- `hash` - Hash index

**Modifiers:** append `:unique` and/or `:name=<identifier>`.

| Modifier | Meaning |
|----------|---------|
| `unique` | Make the index unique |
| `name=`  | Explicit index name for later Ent `StorageKey` emission |

**Examples:**
```go
// +fabrica:field:index
Name string

// +fabrica:field:index=gin
Tags []string

// +fabrica:field:index=btree:unique:name=idx_token_slug
Slug string
```

An unknown modifier is a parse error, so `+fabrica:field:index=btree:uniqe`
fails loudly rather than being ignored.

For an index spanning more than one column, use the resource-level
[`+fabrica:index`](#fabricaindexfieldsf1f2nameiduniquetypetype).

**Database Compatibility:**
| Database   | btree | gin | gist | hash |
|------------|-------|-----|------|------|
| PostgreSQL | ✅    | ✅  | ✅   | ✅   |
| MySQL      | ✅    | ✅  | ❌   | ✅   |
| SQLite     | ✅    | ❌  | ❌   | ❌   |

---

#### `+fabrica:field:default=<value>`

Sets a database-level default value for the field.

**Example:**
```go
// +fabrica:field:default=true
Enabled bool

// +fabrica:field:default=0
RetryCount int
```

**Validation:**
- Cannot combine with `immutable`

---

## API Documentation

### Types

#### `ResourceAnnotations`

Container for all annotations on a resource type.

```go
type ResourceAnnotations struct {
    IsResource     bool                          // Marked with +fabrica:resource
    StorageMode    StorageMode                   // generic or dedicated
    Fields         map[string]*FieldAnnotations  // Field name -> annotations
    SpecFields     map[string]bool               // All parsed Spec field names
    Indexes        []*CompositeIndex             // Resource-level indexes
    Migration      MigrationPolicy               // Migration safety intent
    RawAnnotations []string                      // Original annotation lines
}
```

---

#### `FieldAnnotations`

Container for all annotations on a single field.

```go
type FieldAnnotations struct {
    FieldName      string          // Go field name
    FieldType      string          // Parsed Go type syntax, when available
    Storage        *StorageConfig  // Storage transformation (hashed, encrypted)
    Sensitive      bool            // Exclude from logs
    Immutable      bool            // Prevent updates
    Index          *IndexConfig    // Database index
    Default        string          // Database default value
    Unique         bool            // Unique constraint
    Nullable       bool            // Nullable column intent
    NotNull        bool            // Not-null column intent
    Size           int             // String width cap
    Relation       *RelationConfig // Foreign-key intent
    RawAnnotations []string        // Original annotation lines
}
```

---

#### `StorageConfig`

Defines how a field value is transformed before storage.

```go
type StorageConfig struct {
    Type       StorageType       // default, hashed, encrypted
    Hash       *HashConfig       // Hash parameters (if Type == hashed)
    Encryption *EncryptionConfig // Encryption parameters (if Type == encrypted)
}
```

---

#### `HashConfig`

Hash algorithm parameters.

```go
type HashConfig struct {
    Algorithm HashAlgorithm  // bcrypt, argon2, sha256
    Cost      int            // Algorithm-specific cost parameter
}
```

---

#### `IndexConfig`

Database index parameters.

```go
type IndexConfig struct {
    Type   IndexType  // btree, gin, gist, hash
    Unique bool       // Unique index
    Name   string     // Optional custom index name
}
```

---

### Functions

#### `ParseResourceFile(filename, resourceName) (*ResourceAnnotations, error)`

**Start here.** Returns the complete, validatable annotations for one resource.

A Fabrica resource is split across two declarations: type-level annotations sit
on `<Name>`, field-level annotations on `<Name>Spec`. This merges both, so the
result can be handed straight to `Validate`.

```go
annots, err := annotations.ParseResourceFile("apis/v1/token_types.go", "Token")
if err != nil {
    return err
}
if err := annotations.Validate(annots); err != nil {
    return err
}
```

**Declaration order does not matter** — `TokenSpec` may appear before or after
`Token`. A file that does not declare the resource yields empty annotations
rather than an error; a malformed annotation is a real error, not a silent drop.

Prefer this over calling `ParseResourceAnnotations` per type and merging by
hand, which is easy to get wrong.

---

#### `ParseResourceAnnotations(typeSpec, docComments) (*ResourceAnnotations, error)`

Low-level: parses Fabrica annotations from **one** Go type declaration. It does
not merge `<Name>Spec` — if you call it on `Token` you get the type-level
annotations with no fields. Use `ParseResourceFile` unless you already hold an
`*ast.TypeSpec` and want exactly one declaration's annotations.

**Parameters:**
- `typeSpec` - `*ast.TypeSpec` from `go/ast` parsing
- `docComments` - `*ast.CommentGroup` from parent `GenDecl.Doc` (or `typeSpec.Doc`)

**Returns:**
- `*ResourceAnnotations` - Parsed annotations
- `error` - Parse error if annotation syntax is invalid

**Important:** Type-level comments in Go are attached to `GenDecl.Doc`, not `TypeSpec.Doc`. Always pass `GenDecl.Doc` as the second parameter.

**Example:**
```go
ast.Inspect(file, func(n ast.Node) bool {
    if gd, ok := n.(*ast.GenDecl); ok {
        for _, spec := range gd.Specs {
            if ts, ok := spec.(*ast.TypeSpec); ok {
                annots, err := ParseResourceAnnotations(ts, gd.Doc)
                // ...
            }
        }
    }
    return true
})
```

---

#### `Validate(annotations) error`

Performs semantic validation on parsed annotations.

**Checks:**
- Dedicated storage requires field annotations
- Hash/encryption parameters in valid ranges
- No conflicting annotations (e.g., immutable + default)
- Valid enum values

**Returns:** `error` - Validation error or `nil` if valid

**Example:**
```go
if err := annotations.Validate(annots); err != nil {
    return fmt.Errorf("invalid annotations: %w", err)
}
```

---

#### `ValidateForDatabase(annotations, dbDriver) error`

Validates annotations against database-specific capabilities.

**Parameters:**
- `annotations` - Parsed annotations
- `dbDriver` - Database driver name: `postgres`, `mysql`, `sqlite3`

**Checks:**
- Index type support (SQLite only supports btree)
- Database-specific features

**Returns:** `error` - Validation error or `nil` if compatible

**Example:**
```go
if err := annotations.ValidateForDatabase(annots, "sqlite3"); err != nil {
    return fmt.Errorf("annotations incompatible with SQLite: %w", err)
}
```

---

## Examples

### Token Service with Bcrypt Hashing

```go
// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   TokenSpec   `json:"spec,omitempty"`
    Status TokenStatus `json:"status,omitempty"`
}

type TokenSpec struct {
    // Token value stored as bcrypt hash
    // +fabrica:field:storage=hashed:bcrypt:cost=12
    // +fabrica:field:sensitive
    // +fabrica:field:immutable
    Value string `json:"value"`

    // Indexed for fast lookup
    // +fabrica:field:index
    // +fabrica:field:unique
    Name string `json:"name"`

    // Optional description
    Description string `json:"description,omitempty"`

    // Revocation status (default: false)
    // +fabrica:field:default=false
    Revoked bool `json:"revoked"`
}
```

**Generated Storage:**
```sql
CREATE TABLE tokens (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    namespace     TEXT NOT NULL,
    value         TEXT NOT NULL,  -- bcrypt hash
    description   TEXT,
    revoked       BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMP NOT NULL,
    updated_at    TIMESTAMP NOT NULL,
    UNIQUE(name, namespace)
);

CREATE INDEX idx_tokens_name ON tokens(name);
```

---

### User Service with Encryption

```go
// +fabrica:resource
// +fabrica:storage=dedicated
type User struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   UserSpec   `json:"spec,omitempty"`
}

type UserSpec struct {
    // +fabrica:field:index
    // +fabrica:field:unique
    Email string `json:"email"`

    // +fabrica:field:storage=hashed:bcrypt:cost=14
    // +fabrica:field:sensitive
    Password string `json:"password"`

    // +fabrica:field:storage=encrypted:aes256:key=vault
    // +fabrica:field:sensitive
    SSN string `json:"ssn"`

    // +fabrica:field:default=false
    EmailVerified bool `json:"emailVerified"`
}
```

---

### Document Service with GIN Index

```go
// +fabrica:resource
// +fabrica:storage=dedicated
type Document struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   DocumentSpec `json:"spec,omitempty"`
}

type DocumentSpec struct {
    Title string `json:"title"`

    // Full-text search on content
    // +fabrica:field:index=gin
    Content string `json:"content"`

    // JSON search on tags
    // +fabrica:field:index=gin
    Tags []string `json:"tags"`
}
```

---

### Session Service — composite indexes, sizing, and a relation

A worked example using the full vocabulary together.

```go
// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:migration=additive-only
// Look up a user's sessions newest-first without a filesort:
// +fabrica:index:fields=OwnerID,CreatedAt:name=idx_session_owner_created
// One live session per (owner, device):
// +fabrica:index:fields=OwnerID,DeviceID:unique
type Session struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   SessionSpec `json:"spec,omitempty"`
}

type SessionSpec struct {
    // Opaque session secret: never logged, hashed at rest, set once.
    // +fabrica:field:storage=hashed:sha256
    // +fabrica:field:sensitive
    // +fabrica:field:immutable
    Token string `json:"token"`

    // Owning user. Deleting the user deletes their sessions.
    // +fabrica:field:relation=belongs-to:User:on-delete=cascade
    // +fabrica:field:notnull
    // +fabrica:field:size=36
    OwnerID string `json:"owner_id"`

    // +fabrica:field:size=128
    // +fabrica:field:notnull
    DeviceID string `json:"device_id"`

    // Optional free text supplied by the client.
    // +fabrica:field:nullable
    // +fabrica:field:size=1024
    UserAgent string `json:"user_agent"`

    // +fabrica:field:index
    CreatedAt string `json:"created_at"`
}
```

This records and validates the dedicated-storage intent for later schema
generation:

- `OwnerID` and `UserAgent` string sizes are range-checked.
- composite indexes resolve `OwnerID`, `CreatedAt`, and `DeviceID` against the `Spec` fields.
- `idx_session_owner_created` is checked as a portable index name.
- `token` is checked as a string field using supported SHA-256 hashing intent.
- `additive-only` is checked as a supported migration policy.

The `relation` on `OwnerID` is validated — `on-delete=cascade` is consistent
with `notnull` — but does not yet emit an Ent edge. Schema emission for these
new annotations lands in follow-up stack layers.

---

## Integration Guide

### Step 1: Define Resource with Annotations

```go
// examples/token-service/apis/v1/token_types.go

// +fabrica:resource
// +fabrica:storage=dedicated
type Token struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   TokenSpec   `json:"spec,omitempty"`
}

type TokenSpec struct {
    // +fabrica:field:storage=hashed:bcrypt:cost=12
    Value string `json:"value"`
}
```

---

### Step 2: Parse Annotations in Generator

```go
// pkg/codegen/generator.go

import "github.com/openchami/fabrica/pkg/annotations"

func (g *Generator) processResourceType(typeSpec *ast.TypeSpec, genDecl *ast.GenDecl) error {
    // Parse annotations
    annots, err := annotations.ParseResourceAnnotations(typeSpec, genDecl.Doc)
    if err != nil {
        return fmt.Errorf("parse annotations for %s: %w", typeSpec.Name.Name, err)
    }

    // Validate
    if err := annotations.Validate(annots); err != nil {
        return fmt.Errorf("invalid annotations on %s: %w", typeSpec.Name.Name, err)
    }

    // Database-specific validation
    if err := annotations.ValidateForDatabase(annots, g.config.Database); err != nil {
        return fmt.Errorf("annotations incompatible with %s: %w", g.config.Database, err)
    }

    // Extend metadata with annotations
    meta := &ResourceMetadata{
        Name:             typeSpec.Name.Name,
        StorageMode:      annots.StorageMode,
        FieldAnnotations: annots.Fields,
    }

    // Generate schema based on storage mode
    if meta.StorageMode == annotations.StorageModeDedicated {
        return g.generateDedicatedSchema(meta)
    } else {
        return g.generateGenericSchema(meta)
    }
}
```

---

### Step 3: Generate Dedicated Schema (Phase 3)

```go
// Template: pkg/codegen/templates/ent/schema/resource_dedicated.go.tmpl

func ({{ .Name }}) Fields() []ent.Field {
    return []ent.Field{
        {{- range .Spec.Fields }}
        {{- if .Annotations.Storage }}
        {{- if eq .Annotations.Storage.Type "hashed" }}
        field.String("{{ .JSONName }}").
            Sensitive().
            {{- if .Annotations.Immutable }}
            Immutable().
            {{- end }}
            SchemaType(map[string]string{
                dialect.Postgres: "varchar(60)",  // bcrypt hash length
            }),
        {{- end }}
        {{- else }}
        field.{{ .FieldType }}("{{ .JSONName }}")
            {{- if .Annotations.Unique }}
            .Unique()
            {{- end }}
            {{- if .Annotations.Default }}
            .Default({{ .Annotations.Default }})
            {{- end }},
        {{- end }}
        {{- end }}
    }
}
```

---

## Validation Rules

### Resource-Level

| Rule | Check | Error |
|------|-------|-------|
| Dedicated storage | Must have ≥1 field annotation | "requires at least one field annotation to justify separate table" |
| Storage mode | Must be `generic` or `dedicated` | "unknown storage mode" |
| Migration policy | Must be `unrestricted` or `additive-only` | "unknown migration policy" |
| Composite index storage | Requires dedicated storage | "composite indexes require +fabrica:storage=dedicated" |
| Composite index width | Must cover ≥2 columns | "covers a single column; use +fabrica:field:index on that field instead" |
| Composite index columns | No repeats within one index | "lists field X more than once" |
| Composite index names | Portable identifier, unique per resource | "invalid index name" / "duplicate index name" |
| Composite index fields | Existing exported Go `Spec` fields | "unknown Spec field" / "not a valid Go field name" |

---

### Field-Level

| Annotation | Constraint | Valid Range | Error |
|------------|------------|-------------|-------|
| `bcrypt:cost` | Cost parameter | 4-31 | "bcrypt cost must be 4-31, got N" |
| `argon2:memory` | Memory in KB | 1024-1048576 | "argon2 memory must be 1024-1048576 KB, got N" |
| `encryption:algorithm` | Algorithm name | aes128, aes192, aes256 | "unknown encryption algorithm" |
| `encryption:key` | Key source | env, vault, kms | "unknown key source" |
| `index:type` | Index type | btree, gin, gist, hash | "unknown index type" |
| `index` modifiers | Modifier name | unique, name= | "unknown index modifier" |
| `immutable` + `default` | Conflicting | N/A | "immutable fields should not have database defaults" |
| `nullable` + `notnull` | Conflicting | N/A | "cannot be both nullable and notnull" |
| `size` | Column width | 1-65535, string fields only | "value N out of range [1, 65535]" / "size requires a string field" |
| `relation` kind | Relation kind | belongs-to, has-many | "unknown relation kind" |
| `relation` target | Go type name | identifier | "is not a valid Go type name" |
| `relation:on-delete` | Action | restrict, cascade, set-null | "unknown on-delete action" |
| `on-delete=set-null` without `nullable` | Missing nullability | N/A | "requires +fabrica:field:nullable" |
| `on-delete=set-null` + `notnull` | Conflicting | N/A | "conflicts with +fabrica:field:notnull" |
| `on-delete=set-null` + `immutable` | Conflicting | N/A | "conflicts with +fabrica:field:immutable" |
| `nullable`/`notnull`/`size`/`relation` | Requires dedicated storage | N/A | "X requires +fabrica:storage=dedicated" |

---

## Database Compatibility

### Index Types

| Database   | btree | gin | gist | hash | Notes |
|------------|-------|-----|------|------|-------|
| PostgreSQL | ✅    | ✅  | ✅   | ✅   | All types supported |
| MySQL      | ✅    | ✅  | ❌   | ✅   | No GiST (spatial requires spatial index) |
| SQLite     | ✅    | ❌  | ❌   | ❌   | Only B-tree indexes |

**Validation:** Call `ValidateForDatabase(annots, dbDriver)` to check compatibility.

---

### Hashing Algorithms

| Algorithm | PostgreSQL | MySQL | SQLite | Notes |
|-----------|------------|-------|--------|-------|
| bcrypt    | ✅         | ✅    | ✅     | Recommended (cost=12) |
| argon2    | ✅         | ✅    | ✅     | Modern, memory-hard |
| sha256    | ✅         | ✅    | ✅     | Fast, not for passwords |

---

### Encryption

Encryption is application-level (done before INSERT/after SELECT), so database compatibility is not an issue.

---

## Error Handling

### Error Types

#### `ParseError`

Syntax error in annotation.

```go
type ParseError struct {
    Line    string  // The annotation that failed
    Message string  // Error description
}
```

**Example:**
```
failed to parse annotation "+fabrica:storage=invalid": unknown storage mode "invalid", expected 'generic' or 'dedicated'
```

---

#### `ValidationError`

Semantic error in annotation.

```go
type ValidationError struct {
    Annotation string  // The annotation that failed
    Message    string  // Error description
}
```

**Example:**
```
invalid annotation "field Value bcrypt cost": bcrypt cost must be 4-31, got 3
```

---

### Common Errors

#### "requires at least one field annotation"

```go
// +fabrica:storage=dedicated  // ERROR: no field annotations
type Token struct {
    Name string
}
```

**Fix:** Add at least one field annotation or use `storage=generic`.

---

#### "bcrypt cost must be 4-31"

```go
// +fabrica:field:storage=hashed:bcrypt:cost=32  // ERROR: too high
Value string
```

**Fix:** Use cost between 4-31 (recommend 12-14).

---

#### "immutable fields should not have database defaults"

```go
// +fabrica:field:immutable
// +fabrica:field:default=pending  // ERROR: conflicting
Status string
```

**Fix:** Remove `default` and set value in code.

---

#### "SQLite only supports B-tree indexes"

```go
// +fabrica:field:index=gin  // ERROR: SQLite doesn't support GIN
Content string
```

**Fix:** Use `index` (defaults to btree) or switch to PostgreSQL.

---

## Best Practices

### Security

1. **Always use `sensitive` with hashed/encrypted fields**
   ```go
   // +fabrica:field:storage=hashed:bcrypt
   // +fabrica:field:sensitive
   Password string
   ```

2. **Use higher bcrypt cost for passwords**
   ```go
   // +fabrica:field:storage=hashed:bcrypt:cost=14
   Password string
   ```

3. **Don't hash searchable fields**
   - Hashing prevents lookups (can't find by hashed value)
   - Use encryption instead if searchability needed

---

### Performance

1. **Index frequently queried fields**
   ```go
   // +fabrica:field:index
   Email string
   ```

2. **Use GIN indexes for full-text search**
   ```go
   // +fabrica:field:index=gin
   Content string
   ```

3. **Avoid over-indexing**
   - Indexes slow down writes
   - Only index fields used in WHERE clauses

---

### Maintainability

1. **Group related annotations**
   ```go
   // +fabrica:field:storage=hashed:bcrypt:cost=12
   // +fabrica:field:sensitive
   // +fabrica:field:immutable
   Value string
   ```

2. **Document why annotations are needed**
   ```go
   // Token value must be hashed (PCI compliance)
   // +fabrica:field:storage=hashed:bcrypt:cost=12
   Value string
   ```

3. **Validate early in CI**
   ```bash
   go test ./pkg/annotations/...
   ```

---

## Future Enhancements (Phase 5+)

### Status of the originally planned annotations

Each annotation previously listed here now has an explicit verdict, so there is
one spelling per capability rather than two competing ones.

| Planned | Verdict | Where it went |
|---------|---------|---------------|
| `+fabrica:field:cascade=delete\|null` | **Superseded** | Folded into [`+fabrica:field:relation`](#fabricafieldrelationkindtargeton-deleteaction) as `on-delete=cascade\|set-null`. A referential action is meaningless without a declared relation, so the two belong in one annotation. `cascade=` was never implemented and is **not** accepted. |
| `+fabrica:field:ttl=duration` | **Deferred** | Row expiration needs a runtime reaper, not just a schema change. Nothing to emit onto an Ent field today. |
| `+fabrica:field:computed=expression` | **Deferred** | Ent generated columns need a typed expression model and a dialect-aware emitter; the dedicated template has no branch for it. |
| `+fabrica:field:audit=true` | **Deferred** | Cross-cutting; belongs with the hooks/events machinery rather than the column vocabulary. |

### Still unimplemented

- **Relation emission.** `+fabrica:field:relation` parses and validates, but does
  not yet generate Ent edges. See the annotation's Status note.
- **Schema emission for the new vocabulary.** PR 98 parses and validates
  composite indexes, index modifiers, nullability, size, relations, and
  migration intent. Ent emission for those annotations belongs to follow-up
  stack layers.
- **Migration enforcement.** `+fabrica:migration=additive-only` is recorded as
  intent; the migration tooling does not yet reject non-additive changes.
- **Precision and scale.** No `precision=`/`scale=` annotation exists: the
  dedicated template has no float or decimal branch, so there is nothing to emit
  onto. `size=` covers the string case.
- **Check constraints.** Ent expresses these through dialect-specific
  annotations; deferred until there is a portable way to describe them as intent.

---

## Troubleshooting

### "no type declaration found in source"

**Cause:** Parser can't find the type.

**Fix:** Ensure source is valid Go and use `parser.ParseComments`.

---

### "expected IsResource=true"

**Cause:** Missing `+fabrica:resource` annotation.

**Fix:** Add `// +fabrica:resource` above type.

---

### Tests fail with "expected X, got Y"

**Cause:** Annotation parsing changed behavior.

**Fix:** Check test source strings match annotation format.

---

## Contributing

When adding new annotations:

1. Add enum constants to `types.go`
2. Implement parsing in `parser.go`
3. Add validation in `validator.go`
4. Write tests in `*_test.go`
5. Update this README
6. Run `go test ./pkg/annotations/...`

---

## License

Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
