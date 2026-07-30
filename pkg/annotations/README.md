<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Fabrica Storage Annotations

`github.com/openchami/fabrica/pkg/annotations` parses, resolves, and validates the closed annotation contract used by Fabrica's generated dedicated Ent storage. Generation fails before committing output when a directive is malformed, unknown, incompatible with its Go type, or unsupported by the selected database.

## Supported workflow

```go
// +fabrica:resource
// +fabrica:storage=dedicated
type User struct {
    APIVersion string
    Kind       string
    Metadata   fabrica.Metadata
    Spec       UserSpec
    Status     UserStatus
}

type UserSpec struct {
    // +fabrica:field:unique
    // +fabrica:field:index=btree
    Email string `json:"email" validate:"required"`

    // +fabrica:field:storage=hashed:bcrypt:cost=12
    // +fabrica:field:sensitive
    Password string `json:"password" validate:"required"`

    // +fabrica:field:immutable
    Username string `json:"username" validate:"required"`
}
```

The normal `fabrica generate` registration path discovers these comments, resolves them for the configured dialect, and selects one authoritative storage route per resource. Dedicated resources use only their dedicated Ent entity; generic resources use only the shared JSON resource entity. Routing is exclusive, not dual-write.

Field directives require dedicated Ent storage. Any field-level storage, SQL, constraint, default, sensitive, or immutable directive on a generic resource fails before managed output changes, whether the selected backend is Ent or file. Dedicated mode with file storage is rejected for the same reason: the backend cannot enforce the dedicated contract.

## Tested capability matrix

Every supported row names the executable test that proves it. A parser or template branch alone is not a supported capability.

| Capability | PostgreSQL | SQLite | Executable evidence |
|---|---|---|---|
| Fields: `string`, `bool`, `int`, `int64`, `float64`, `time.Time`, and `[]string` | Supported | Supported | `TestCapabilities_supports_closed_field_matrix` |
| Nillable pointers: `*string`, `*bool`, `*int`, `*int64`, `*float64`, `*time.Time` | Supported | Supported | `TestCapabilities_supports_ent_nillable_scalar_pointers` |
| Bcrypt on `string` and `*string` | Supported | Supported | `TestCapabilities_supports_bcrypt` |
| Field directives require dedicated Ent storage | Fail closed | Fail closed | `TestPrepareResourceAnnotations_rejects_field_directives_when_storage_cannot_enforce_them` |
| Dedicated mode with file storage | Rejected | Rejected | `TestPrepareResourceAnnotations_rejects_dedicated_mode_for_file_backend` |
| File-backed resource version snapshots | File backend only | File backend only | `TestGeneratedFileVersioning_builds_and_runs_snapshot_runtime` |
| Ent resource version snapshots | Rejected before output | Rejected before output | `TestPrepareResourceAnnotations_rejects_ent_version_snapshots_for_every_storage_mode` |
| Required bcrypt create and omitted update | Supported | Supported | `TestDedicatedSecurity_generated_adapter_runtime` |
| Sensitive zero-value update semantics | Supported | Supported | `TestDedicatedSecurity_generated_adapter_runtime` |
| Persisted redacted write responses | Supported | Supported | `TestDedicatedSecurity_generated_adapter_runtime` |
| Scalar defaults: string, bool, int, int64, float64 | Supported | Supported | `TestGeneratedDedicatedSchema_default_modifiers_match_pointer_shape` |
| Unique constraints | Supported | Supported | `TestGeneratedDedicatedIndex_baseline_portable_btree_and_unique` |
| Portable B-tree indexes | Supported | Supported | `TestGeneratedDedicatedIndex_baseline_portable_btree_and_unique` |
| GIN index on `[]string` | Supported | Rejected | `TestGeneratedDedicatedIndex_postgresql_methods_use_ent_annotations` |
| Hash index on scalar fields | Supported | Rejected | `TestGeneratedDedicatedIndex_postgresql_methods_use_ent_annotations` |
| Complete resource envelope | Supported | Supported | `TestDedicatedEnvelope_schema_renders_complete_envelope` |
| Exclusive generic/dedicated CRUD routing | Supported | Supported | `TestDedicatedStorageRouting_generated_helpers_have_authoritative_callers` |
| Explicit non-destructive migration helpers | Supported | Supported | `TestGeneratedMigration_is_explicit_and_dedicated_only` |
| Unique create/update conflict response | HTTP 409 | HTTP 409 | `TestGeneratedHandlers_map_create_and_update_storage_conflicts_to_stable_409` |
| Backend-common typed conflict contract | Supported | Supported | `TestGeneratedStorageErrors_define_backend_independent_conflict_contract` |
| Commit-aware migration continuation cursor | Supported | Supported | `TestDedicatedMigration_generated_SQLite_runtime` |
| Generated project generation, vet, and build | Supported | Supported | `TestGeneratedProjectMatrix_passes_generation_vet_and_build` |
| Generated SQLite runtime | N/A | Supported | `TestGeneratedSQLite_acceptance` |
| Restricted-role PostgreSQL runtime | Supported | N/A | `TestGeneratedPostgres_acceptance` |

PostgreSQL and SQLite are the only supported dialects for annotation-driven dedicated storage.

## Field contract

### Types and nullability

The closed non-pointer set is `string`, `bool`, `int`, `int64`, `float64`, `time.Time`, and `[]string`. Supported pointers are `*string`, `*bool`, `*int`, `*int64`, `*float64`, and `*time.Time`. A pointer becomes an Ent `Optional().Nillable()` field and preserves absence separately from the scalar zero value. `*[]string`, maps, nested structs, named aliases, arbitrary slices, and other Go types are rejected with a typed capability error.

### Defaults

`+fabrica:field:default=<literal>` supports string, bool, int, int64, and finite float64 values. Defaults are parsed before rendering, so malformed, overflowing, NaN, and infinity literals fail generation with source context. A pointer may combine nullability with a database default. `time.Time`, `[]string`, and transformed fields do not support defaults. Defaults and `immutable` are mutually exclusive.

### Storage transform and sensitive output

`+fabrica:field:storage=hashed:bcrypt[:cost=N]` is supported for `string` and `*string`; bcrypt is the only supported storage transform. Cost must be in bcrypt's 4–31 range. A required bcrypt value must be present and non-empty on create. On update, an omitted value or the redacted zero value preserves the stored hash; supplied nonzero plaintext replaces it with a new hash. An existing bcrypt value is not hashed a second time.

`+fabrica:field:sensitive` zeroes the field at the API boundary. The JSON key remains present with the type's zero value; it is not omitted. Bcrypt and API zeroing are separate guarantees: the database stores a bcrypt hash, while API responses expose neither plaintext nor hash.

Dedicated updates interpret a redacted non-pointer zero value as omitted: empty strings, `false`, numeric zero, zero `time.Time`, and empty or nil `[]string` preserve the stored value. A nonzero value explicitly replaces it. When an explicit zero replacement is required, use the corresponding supported pointer type and send a non-nil pointer to zero; nil preserves the stored value. `[]string` has no supported pointer form, so an empty sensitive slice cannot explicitly clear storage.

After a successful dedicated create, PUT, or PATCH, the generated handler reloads the persisted entity and converts it through the redacting adapter before publishing or returning it. The response therefore reflects database defaults and immutable values while exposing neither submitted sensitive values nor stored hashes. Status handlers remain status-only writes.

### Immutable, unique, and indexes

- `+fabrica:field:immutable` preserves the stored value on update. It does not promise HTTP 422.
- `+fabrica:field:unique` creates a database uniqueness constraint. Every generated backend compiles against the same backend-common typed conflict contract. Ent classifies constraint failures through that contract, and generated create and update handlers map them to HTTP 409 with a stable storage-conflict response. File storage does not invent uniqueness constraints.
- `+fabrica:field:index` and `index=btree` are portable.
- PostgreSQL additionally supports `index=gin` for `[]string` and `index=hash` for scalar fields.
- SQLite rejects GIN and hash before rendering.

## Complete envelope and routing

Dedicated storage preserves the complete resource envelope: API version, kind, identity, namespace, UID, labels, annotations, resource version, creation/update timestamps, Spec, and Status. Conversion errors stop the operation rather than dropping envelope state.

Each resource has one storage authority. Dedicated CRUD and query helpers never fall through to the generic `ent.Resource` path. Regenerating a resource from dedicated to generic removes managed dedicated artifacts before Ent regeneration, preventing stale entities from remaining selectable.

## Migration and cutover

Generation emits preview and migration helpers only for dedicated resources. Migration is explicit, opt-in, and non-destructive: it is never called by server startup or generation, and it does not delete source rows from the generic table. Operators must:

Continuation cursors are commit-aware. Preview publishes its cursor only after a complete successful preview. A write batch advances its cursor only after the transaction commits; conversion, hashing, constraint, cancellation, or commit failures roll back and return the input `AfterID` with `Copied=0`, so retry starts from the last durable boundary.

1. back up the database;
2. preview and resolve conflicts;
3. run the generated migration under the intended application role;
4. verify counts and application reads;
5. cut traffic to the dedicated route; and
6. retain or remove generic rows only under a separate operator-controlled retention procedure.

The generated managed-schema transaction stages output, uses a process/kernel lock, and restores the prior managed tree after an interrupted or failed replacement. These filesystem guarantees do not replace a database backup or an operator-approved cutover.

## Strict diagnostics

The supported public resolution entry points are:

```go
resolved, err := annotations.ResolveStorageIntent(filename, "User", annotations.DialectPostgreSQL)
resolved, err := annotations.ResolveStorageIntentFromReflect(resourceType, source, annotations.DialectSQLite)
```

`CapabilityError`, `ParseError`, and `DefaultError` retain source location and semantic context, including filename, line, column, resource type, field, directive, and error category where applicable. Unknown or malformed Fabrica directives and unsupported security/integrity requests fail before generated output is committed. Cache hits clone complete resolved metadata and preserve the same diagnostics as cold parsing.

`ParseResourceAnnotations` and `Validate` remain lower-level parsing APIs. Code generation should use the resolved storage intent rather than treating parse acceptance as capability support.

## Rejected capabilities

Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected. Specifically:

| Capability | Contract | Executable evidence |
|---|---|---|
| Encryption | Rejected before output; no encryption or key-management runtime | `TestUnsupportedCapabilities_return_typed_source_error` |
| Argon2 | Rejected before output | `TestUnsupportedCapabilities_return_typed_source_error` |
| SHA-256 storage hashing | Rejected before output | `TestUnsupportedCapabilities_return_typed_source_error` |
| MySQL dedicated annotations | Rejected as an unsupported dialect | `TestUnsupportedCapabilities_rejects_mysql_dialect` |
| GiST | Rejected for every supported dialect | `TestSecurityDialect_unsupported_crypto_types_and_indexes_return_capability_errors` |
| Unsupported Go types | Rejected with a typed capability error | `TestUnsupportedCapabilities_return_typed_source_error` |
| Unknown Fabrica directives | Rejected with a source-located typed parse error | `TestParseFileAnnotations_rejects_invalid_directives_with_source_context` |

GIN on scalar fields, hash on `[]string`, bcrypt on non-string fields, unsupported pointers, maps, nested structs, named aliases, `[]byte`, and arbitrary slices are also rejected by the typed resolver.

There is no compatibility mode that silently ignores these requests and no unsafe override.

## Verification

```bash
go test -race -shuffle=on -count=1 ./pkg/annotations ./pkg/codegen
go test -tags=integration -count=1 -v ./pkg/codegen -run '^TestGeneratedPostgres'
bash examples/12-storage-annotations/verify-example.sh
```

Example 12 is the executable SQLite walkthrough. PostgreSQL behavior belongs to the restricted-role integration suite, not to claims inferred from SQLite.
