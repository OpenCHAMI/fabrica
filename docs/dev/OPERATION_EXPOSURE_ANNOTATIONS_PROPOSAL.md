<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Operation and Exposure Annotations Proposal

**Status:** Implemented
**Scope:** Implemented annotation parsing, validation, resolution, and generated server/OpenAPI/client filtering. Handler extension hooks remain a separate future proposal.

## Problem

Fabrica's generated server routes currently expose the full resource CRUD surface for every generated resource: list, get, create, update, patch, delete, status update, and status patch. Generated OpenAPI and generated clients mirror that surface. This is safe for ordinary public API resources, but not for resources whose state must only change through domain-specific transaction boundaries.

TokenSmith is the motivating example. It can model `BootstrapTokenPolicy`, `RefreshTokenFamily`, and `RefreshTokenGeneration` as hash-free resources, but raw generated CRUD would let callers create, mutate, or delete token-state rows outside the existing OAuth transaction rules. That would bypass one-time bootstrap redemption, refresh-token replay revocation, immutable grant enforcement, and explicit PostgreSQL migration authority.

Fabrica needs a regeneration-safe way for a resource author to say which generated HTTP operations exist and whether the resource is public, protected, internal, or private.

## Goals

- Allow resources to opt out of unsafe generated CRUD operations without hand-editing generated files.
- Keep generated server routes, OpenAPI, and clients consistent with the same operation policy.
- Preserve backward compatibility for existing resources that do not use the new annotations.
- Make private/internal resources still usable for API-version metadata, storage generation, and handwritten integration code.
- Fail closed on unknown verbs, unknown exposure values, or contradictory annotations.

## Non-Goals

- Implement custom business logic in annotations.
- Replace service-specific authorization middleware.
- Make unsafe token-state CRUD safe by redaction alone.
- Change storage semantics, migration behavior, or generated Ent schemas.
- Define arbitrary SQL, transactions, relationships, or action endpoints.

## Proposed Annotation Contract

### Resource Verbs

```go
// +fabrica:verbs=list,get
// +fabrica:resource
type RefreshTokenFamily struct { ... }
```

`+fabrica:verbs=<csv>` selects the generated HTTP operations for a resource. Supported values:

| Verb | Generated route |
| --- | --- |
| `list` | `GET /resources` |
| `get` | `GET /resources/{uid}` |
| `create` | `POST /resources` |
| `update` | `PUT /resources/{uid}` |
| `patch` | `PATCH /resources/{uid}` |
| `delete` | `DELETE /resources/{uid}` |
| `statusUpdate` | `PUT /resources/{uid}/status` |
| `statusPatch` | `PATCH /resources/{uid}/status` |
| `versionList` | `GET /resources/{uid}/versions` when resource versioning is enabled |
| `versionGet` | `GET /resources/{uid}/versions/{versionID}` when resource versioning is enabled |
| `versionDelete` | `DELETE /resources/{uid}/versions/{versionID}` when resource versioning is enabled |
| `all` | Compatibility alias for every operation currently generated |
| `none` | No generated HTTP operations |

Rules:

- Missing `+fabrica:verbs` preserves current generation behavior, equivalent to `all`.
- `none` must appear alone.
- `all` must appear alone.
- Version verbs are invalid unless resource versioning is enabled.
- Unknown verbs are errors before generated output is written.
- Disabled operations are omitted from generated routes, handlers, OpenAPI, and generated clients.

### Resource Exposure

```go
// +fabrica:exposure=private
// +fabrica:resource
type BootstrapTokenPolicy struct { ... }
```

`+fabrica:exposure=<value>` classifies how the resource may be exposed through generated artifacts.

| Exposure | Meaning |
| --- | --- |
| `default` | Backward-compatible behavior; generated routes are registered exactly as they are today. |
| `public` | Generated routes are intended for unauthenticated/public registration by the application. |
| `protected` | Generated routes are intended for normal authenticated/authorized registration by the application. |
| `internal` | Generated routes may be emitted into a separate internal registration function, but must not appear in public OpenAPI or public clients. |
| `private` | No generated HTTP routes, OpenAPI paths, or client methods. Non-HTTP artifacts such as API-version metadata and storage may still be generated. |

Rules:

- Missing `+fabrica:exposure` preserves current generation behavior, equivalent to `default`.
- `private` implies `verbs=none` for server, OpenAPI, and client output.
- `internal` requires generated output to separate public/protected route registration from internal route registration.
- `public` and `protected` do not implement auth by themselves; they classify generated output so application wiring can mount route groups under the correct middleware.
- Unknown exposure values are errors before generated output is written.

## Generator Behavior

The operation/exposure policy should be resolved once and reused by every generator surface.

| Generator surface | Required behavior |
| --- | --- |
| Server routes | Register only enabled operations. Omit private resources entirely. |
| Server handlers | Emit handlers only for enabled operations, or emit unreferenced handlers only if a compatibility flag explicitly requests legacy output. |
| OpenAPI | Document only routes generated for public/protected exposure. Exclude private and internal resources by default. |
| Client | Generate methods only for public/protected routes in OpenAPI. Do not generate clients for private/internal routes by default. |
| Storage | Unchanged. Storage generation is governed by storage annotations and project config. |
| API-version registry | Include resources even when exposure is private, because registry metadata is not route exposure. |

## TokenSmith Example

TokenSmith's current token-state resources would use private exposure initially:

```go
// +fabrica:resource
// +fabrica:storage=dedicated
// +fabrica:exposure=private
// +fabrica:verbs=none
type RefreshTokenFamily struct { ... }
```

That would let Fabrica generate API-version metadata and storage/projection artifacts while preventing generated public CRUD, generated OpenAPI CRUD paths, and generated client CRUD methods.

If TokenSmith later wants operator read-only projections, it could choose:

```go
// +fabrica:exposure=protected
// +fabrica:verbs=list,get
```

That still would not permit create, update, patch, delete, or status mutation. Domain mutations would remain handwritten action endpoints that call TokenSmith's existing transaction layer.

## Required Tests

- Parser accepts valid verb lists and exposure values.
- Parser rejects unknown verbs, unknown exposure values, `none` with other verbs, and `all` with other verbs.
- Generated routes omit disabled operations.
- Generated OpenAPI and client output match the route policy.
- Private resources generate no HTTP/OpenAPI/client surface while remaining present in API-version metadata.
- Existing resources without annotations produce byte-equivalent or behavior-equivalent current CRUD output.
- A TokenSmith-like fixture proves private token-state resources do not produce CRUD routes, OpenAPI paths, or client methods.

## Implemented Decisions

- Internal routes are emitted through `RegisterGeneratedInternalRoutes` and are not mounted automatically.
- Public routes use `RegisterGeneratedPublicRoutes`; protected and default routes use `RegisterGeneratedProtectedRoutes`.
- `RegisterGeneratedRoutes` remains a compatibility wrapper for public plus protected/default routes.
- The generated OpenAPI document and generated clients include only public, protected, and default resources.
- Status and version operations are independently selectable verbs.
- Operation policy is annotation-only in this implementation; there is no `apis.yaml` override or internal OpenAPI mode.
- Private exposure without explicit verbs resolves to none. Explicit private exposure with any verbs other than `none` is invalid.
- Generated handler extension hooks are not part of this implementation.
