// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

/*
Package annotations resolves Fabrica storage directives into a closed, typed
contract for generated dedicated Ent storage.

The normal generation path recognizes resource storage selection and field
directives for defaults, nullability, bcrypt, sensitivity, immutability,
uniqueness, and indexes. Field directives require dedicated Ent storage;
generic Ent/file resources and dedicated resources on the file backend fail
closed before output. Annotation-driven dedicated storage supports PostgreSQL
and SQLite. It uses exclusive routing: a dedicated resource uses its dedicated
Ent entity, while a generic resource uses the shared JSON entity.

Resource version snapshots are file-backend only. Generic and dedicated Ent
resources with versioning enabled fail before output. Unknown Fabrica
directives fail with a source-located typed parse error rather than being
ignored.

# Capability contract

Supported fields are string, bool, int, int64, float64, time.Time, and
[]string. Supported pointers are *string, *bool, *int, *int64, *float64, and
*time.Time. Pointer fields are optional and nillable. Scalar defaults support
string, bool, int, int64, and finite float64 literals; time.Time, []string,
and transformed defaults are rejected.

Bcrypt is the only supported storage transform and accepts string or *string.
A required bcrypt value must be non-empty on create; omission or a redacted
zero on update preserves storage. Sensitive fields are zeroed at the API
boundary without omitting their JSON keys. Non-pointer sensitive zeros
preserve storage, while non-nil pointers explicitly replace it, including
with zero. Dedicated writes reload persisted entities before producing
redacted write responses. Immutable fields preserve their stored value.

Every generated backend compiles against one backend-common conflict contract.
Ent constraint causes map through it to HTTP 409; file storage remains
unconstrained.

Portable indexes use B-tree. PostgreSQL additionally supports GIN for
[]string and hash for scalar fields. SQLite accepts only B-tree indexes.

Dedicated conversion preserves the complete resource envelope, including
identity, namespace, UID, labels, annotations, resource version, timestamps,
Spec, and Status.

# Unsupported requests

Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected.
There is no plaintext encryption fallback, algorithm downgrade, compatibility
ignore mode, or unsafe override.

# Diagnostics

ResolveStorageIntent and ResolveStorageIntentFromReflect return typed errors
with source and semantic context. Malformed, unknown, or unsupported directives
fail before generated output is committed. Parse acceptance alone does not
establish backend support.

# Migration

Generated migration helpers are explicit and non-destructive. They are not
called by generation or server startup and do not delete generic source rows.
Preview publishes a cursor only after successful completion. Write cursors
advance only after commit; rollback returns the input cursor and zero copied
rows.
Operators own backup, preview, conflict resolution, verification, traffic
cutover, and any later source-data retention action.

The Tested capability matrix and its evidence linkage are maintained in README.md. Its
principal gates include TestCapabilities_supports_closed_field_matrix,
TestCapabilities_supports_ent_nillable_scalar_pointers,
TestCapabilities_supports_bcrypt,
TestUnsupportedCapabilities_return_typed_source_error,
TestGeneratedProjectMatrix_passes_generation_vet_and_build,
TestGeneratedSQLite_acceptance, and TestGeneratedPostgres_acceptance.
*/
package annotations
