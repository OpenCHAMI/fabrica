<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Generated Handler Extension Hooks Proposal

**Status:** Proposed
**Scope:** Design proposal only; no implementation is approved by this document.

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

The generated file calls a package-level hook provider from the create-once file:

```go
var refreshTokenFamilyHooks RefreshTokenFamilyHooks = NoopRefreshTokenFamilyHooks{}
```

The hook interface is generated from enabled operations only. A conceptual shape:

```go
type RefreshTokenFamilyHooks interface {
    BeforeList(context.Context, *http.Request) error
    AfterList(context.Context, *http.Request, []v1.RefreshTokenFamily) ([]v1.RefreshTokenFamily, error)

    BeforeGet(context.Context, *http.Request, string) error
    AfterGet(context.Context, *http.Request, *v1.RefreshTokenFamily) (*v1.RefreshTokenFamily, error)

    BeforeCreate(context.Context, *http.Request, *CreateRefreshTokenFamilyRequest) error
    ExecuteCreate(context.Context, *http.Request, *CreateRefreshTokenFamilyRequest) (*v1.RefreshTokenFamily, bool, error)
    AfterCreate(context.Context, *http.Request, *v1.RefreshTokenFamily) (*v1.RefreshTokenFamily, error)
}
```

`Execute*` hooks return `(resource, handled, error)`:

- `handled=false` means the generated handler continues with generated storage behavior.
- `handled=true` means the hook has completed the operation and the generated handler skips generated storage.
- `error` is mapped through the generated error response path.

Only operations enabled by the operation/exposure policy get hook methods. A disabled operation has no route and no hook call.

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

Executor hooks should be opt-in per operation and visible in generated code. A project-level setting may be required before generator emits executor hook calls, so ordinary generated services keep the simpler default path.

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
- Non-typed hook errors should use the existing generated error behavior.
- Sensitive internal causes must not be serialized into public responses.

## Regeneration Contract

- Generated files may call hook interfaces and default no-op implementations.
- Create-once hook files are never overwritten after first generation.
- Adding a new enabled operation may add a new hook method. This is a compile-time signal that user-owned hook implementations must be updated.
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

Then read hooks can filter, authorize, and redact projection data while create/update/delete remain absent. If TokenSmith later defines generated action support, executor hooks can delegate action semantics to `TokenStateStore` instead of direct storage CRUD.

## Required Tests

- Generated handlers call no-op hooks without changing existing behavior.
- A create-once hook file is generated only when absent and preserved across regeneration.
- Guard hook errors stop the generated operation before storage mutation.
- Response hooks can transform a response without mutating storage.
- Executor hooks can replace storage for an enabled operation.
- Disabled operations do not call hooks and do not appear in routes/OpenAPI/client output.
- Typed hook errors map to expected HTTP status codes without leaking internal causes.
- A TokenSmith-like fixture proves replay-sensitive operations can be delegated to a handwritten transaction service without generated storage mutation.

## Open Questions

- Should hook interfaces be generated per resource or as generic typed interfaces in a shared package?
- Should executor hooks be emitted by default, or gated behind a project config option?
- Should hook stubs live in `cmd/server` or in an internal package for easier unit testing?
- How should hook interface changes be versioned to avoid surprising compile failures during regeneration?
- Should OpenAPI support documenting hook-backed action endpoints, or should that be a separate action-endpoint proposal?
