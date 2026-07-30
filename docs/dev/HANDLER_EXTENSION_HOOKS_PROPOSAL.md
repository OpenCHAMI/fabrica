<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Generated Handler Extension Hooks Proposal

**Status:** Implemented
**Scope:** Implement regeneration-safe generated handler hooks for enabled generated operations. Operation/exposure annotations remain the source of truth for whether a route exists; disabled or private operations do not get routes or hook calls.

## Problem

Fabrica generated handlers currently decode requests, validate resources, call generated storage functions, publish events, and return responses. The only stable regeneration-safe customization points are outside the handlers: middleware around route registration and create-once OpenAPI extensions. That is not enough for resources whose safe behavior depends on domain-specific checks or transaction boundaries at the operation itself.

TokenSmith is the motivating example. Even if Fabrica can hide token hashes from resource JSON, generated create/update/delete handlers still call storage directly. TokenSmith must instead redeem bootstrap tokens and rotate refresh tokens through `TokenStateStore.RedeemBootstrap` and `TokenStateStore.RotateRefresh`, which run serializable PostgreSQL transactions and enforce replay revocation.

Fabrica needs regeneration-safe handler hooks that let applications attach authorization, validation, transformation, response filtering, and optional operation execution without editing generated files.

## Goals

- Preserve generated handler regeneration safety.
- Keep generated default behavior unchanged when no hooks are implemented.
- Allow handwritten code to intercept or reject operations before storage mutation.
- Allow handwritten code to delegate an operation to an authoritative domain transaction instead of generated storage when explicitly configured.
- Keep hook signatures stable and typed enough for compile-time checking.
- Ensure generated OpenAPI/client behavior is still driven by operation/exposure policy, not by hook implementation side effects.

## Non-Goals

- Add arbitrary plugin loading or runtime scripting.
- Put business logic in annotations.
- Make hooks a substitute for route-level authentication middleware.
- Let hooks silently re-enable operations disabled by operation/exposure annotations.
- Require every generated service to implement hook files.

## Proposed Generated Shape

For each generated resource, Fabrica writes normal generated handlers and creates a companion create-once hook stub if it does not already exist:

```text
cmd/server/<resource>_handlers_generated.go      # overwritten
cmd/server/<resource>_hooks.go                   # create-once, user-owned
```

The generated file defines the evolving optional-function hook type. The create-once file owns only the stable package variable, so adding an enabled operation does not require overwriting user code:

```go
var refreshTokenFamilyHooks = RefreshTokenFamilyHooks{}
```

Hook fields are generated from enabled operations only. A resource with no enabled HTTP operations or private exposure receives no hook file, fields, or calls. A conceptual shape:

```go
type RefreshTokenFamilyHooks struct {
    BeforeList func(context.Context, *http.Request) error
    AfterList func(context.Context, *http.Request, http.Header, []*v1.RefreshTokenFamily) ([]*v1.RefreshTokenFamily, error)

    BeforeGet func(context.Context, *http.Request, string) error
    AfterGet func(context.Context, *http.Request, http.Header, *v1.RefreshTokenFamily) (*v1.RefreshTokenFamily, error)

    BeforeCreate func(context.Context, *http.Request, *CreateRefreshTokenFamilyRequest) error
    ExecuteCreate func(context.Context, *http.Request, *CreateRefreshTokenFamilyRequest) (*v1.RefreshTokenFamily, bool, error)
    AfterCreate func(context.Context, *http.Request, http.Header, *v1.RefreshTokenFamily) (*v1.RefreshTokenFamily, error)
}
```

`Execute*` hooks return `(resource, handled, error)`:

- `handled=false` means the generated handler continues with generated storage behavior.
- `handled=true` means the hook has completed the operation and the generated handler skips generated storage.
- `error` is mapped through the generated error response path.

Only operations enabled by the operation/exposure policy get hook methods. A disabled operation has no route and no hook call. Hooks cannot create a route, OpenAPI path, or client method that operation/exposure annotations disabled.

## Hook Categories

### Guard Hooks

Guard hooks run before generated storage mutation. They can reject a request but cannot replace storage behavior.

Examples:

- authorization checks that require parsed request values;
- cross-field validation not expressible as struct tags;
- tenant/namespace guardrails;
- read filtering decisions.

### Response Hooks

Response hooks run after generated storage has loaded or persisted a resource and before the HTTP response is written.

Examples:

- redaction beyond generated sensitivity behavior;
- adding response headers;
- hiding internal status fields;
- emitting application-specific audit records.

### Executor Hooks

Executor hooks can replace generated storage for a specific enabled operation. They are for resources whose operation must call a domain service or transactional repository.

Examples:

- TokenSmith bootstrap redemption through `RedeemBootstrap`;
- TokenSmith refresh rotation through `RotateRefresh`;
- a power-control transition create that maps to an external state machine;
- a resource whose delete operation must enqueue a safe reconciliation task instead of deleting a row.

Executor hooks are generated for enabled mutating operations, but they are no-ops by default. They may replace generated storage only when user-owned hook code returns `handled=true`. This keeps ordinary generated services on the existing storage path unless user code explicitly takes ownership of an operation.

## Error Mapping

Generated code should not force every hook error to HTTP 500. Hooks need a typed, stable error contract:

```go
type HandlerError struct {
    StatusCode int
    PublicMessage string
    Cause error
}
```

Rules:

- `errors.Is` and `errors.As` should preserve wrapped domain errors.
- Generated handlers should map `HandlerError` status codes directly.
- Only statuses from 400 through 599 are accepted; invalid statuses safely map to 500.
- Non-typed hook errors should use the existing generated error behavior.
- Sensitive internal causes must not be serialized into public responses.

## Regeneration Contract

- Generated files define optional-function hook sets; nil fields are the no-op implementation.
- Create-once hook files are never overwritten after first generation.
- Adding a new enabled operation may add a new optional function field on the generated hook set without changing the create-once variable declaration.
- Removing an operation removes generated calls to the hook but does not rewrite user-owned files.
- Hooks cannot make disabled routes reachable.
- Hook signatures are versioned as part of the generated API contract.

## Interaction With Operation/Exposure Annotations

Operation/exposure policy decides whether a route, OpenAPI path, client method, handler, and hook call exists. Handler hooks decide what happens inside a generated operation that exists.

For TokenSmith, the initial safe state is:

```go
// +fabrica:exposure=private
// +fabrica:verbs=none
```

No generated handler hooks are needed because no generated HTTP operation exists. If TokenSmith later exposes a protected read-only projection:

```go
// +fabrica:exposure=protected
// +fabrica:verbs=list,get
```

Then read hooks can filter, authorize, and redact projection data while create/update/delete remain absent. If TokenSmith later enables a protected mutating operation, executor hooks can delegate the operation to `TokenStateStore` instead of direct storage CRUD. Action-style endpoints remain a separate design problem; this implementation only affects generated resource operations that already exist.

## Implementation Decisions

- Hooks are generated as create-once files beside generated handlers, for example `cmd/server/token_hooks.go`.
- Generated handler files are overwritten as usual and call hook variables declared in the create-once hook file.
- Hook stubs use optional function fields rather than forcing users to implement a large interface. Missing fields mean no-op/default generated behavior.
- Generated no-op hook sets are safe to embed in user-owned hook implementations.
- Guard hooks run before storage interaction for their operation.
- Response hooks run after generated storage behavior or executor behavior and before the response is serialized.
- Executor hooks exist only for mutating operations that can be replaced without changing route shape: create, update, patch, delete, status update, and status patch. Read operations use guard/response hooks only.
- Hook errors use generated `HandlerError` helpers for stable status mapping. Unwrapped errors keep existing generated behavior.
- Operation/exposure policy decides whether hook code is emitted for an operation. Private resources and `verbs=none` resources do not receive hook methods or calls.

## Required Tests

- Generated handlers call no-op hooks without changing existing behavior.
- A create-once hook file is generated only when absent and preserved across regeneration.
- Guard hook errors stop the generated operation before storage mutation.
- Response hooks can transform a response without mutating storage.
- Executor hooks can replace storage for an enabled operation.
- Disabled operations do not call hooks and do not appear in routes/OpenAPI/client output.
- Typed hook errors map to expected HTTP status codes without leaking internal causes.
- A TokenSmith-like fixture proves replay-sensitive operations can be delegated to a handwritten transaction service without generated storage mutation.

## Future Follow-ups

- How should hook interface changes be versioned to avoid surprising compile failures during regeneration?
- Should OpenAPI support documenting hook-backed action endpoints, or should that be a separate action-endpoint proposal?
