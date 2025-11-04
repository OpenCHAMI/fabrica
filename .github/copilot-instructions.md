<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Copilot Instructions for Fabrica

Use these repo-specific guidelines when proposing changes, generating code, or running commands.

## Big picture
- Fabrica is a Go code generator for REST APIs with Kubernetes-style resources (APIVersion, Kind, Metadata, Spec, Status).
- Key packages: `pkg/codegen` (generator), `pkg/codegen/templates` (embedded via go:embed), core libs (`pkg/resource`, `pkg/validation`, `pkg/conditional`, `pkg/events`, `pkg/reconcile`, `pkg/storage`).
- CLI entrypoint: `cmd/fabrica`.
- Examples and docs illustrate intended usage: `examples/`, `docs/`.

## Repository map (what to edit vs. not)
- DO edit: templates in `pkg/codegen/templates/**`, CLI in `cmd/fabrica/**`, core libs in `pkg/**`, docs in `docs/**`, examples in `examples/**`.
- DON’T edit: generated files in user projects (anything ending with `_generated.go`). Generated projects keep custom code in resource files and `cmd/server/main.go` before the first `// Generated` marker.

## Core developer workflows
- Build CLI: `make build` (outputs `bin/fabrica`).
- Template changes: rebuild CLI, then regenerate a sample project to validate.
- Create test project (local templates):
  1) `fabrica init <name> [--events --events-bus memory]`
  2) `fabrica add resource <Resource>`
  3) `fabrica generate`
  4) Add replace to project’s `go.mod` to test local fabrica: `go mod edit -replace github.com/openchami/fabrica=/path/to/local/fabrica` then `go mod tidy`
  5) Run server: `go run ./cmd/server/` (trailing slash matters because there are multiple files).
- Lint: `golangci-lint run` (config in `.golangci.yaml`). Examples and `test/integration` are excluded via the config.
- CI locally (optional): `make act-install && make act-all`.

## Code generation contract (important)
- Resource definitions live in user projects under `pkg/resources/<name>/<name>.go` and embed `resource.Resource`.
- Regeneration command in user projects: `fabrica generate` (idempotent). Never hand-edit `*_generated.go`.
- List endpoints return a flat JSON array (not a Kubernetes-style `{items: [...]}` object).
- Status subresource pattern is supported (see `docs/status-subresource.md`) — spec and status updates are distinct.

## Events and reconciliation
- Event bus: use `events.NewInMemoryEventBus(buffer, workers)` for local dev. `Subscribe(eventType, handler)` returns `(SubscriptionID, error)`. `Close()` has no context arg.
- Publish via functions in `pkg/events` (e.g., `PublishResourceCreated`); don’t construct CloudEvents directly in generated server code.
- Use `resource.Condition` for status condition changes. Condition change events are emitted when enabled.

## Storage
- File backend lives in `pkg/storage/file_backend.go`. Ent integration is documented in `docs/storage-ent.md`. Some examples require SQLite foreign keys (`?_fk=1`) and ensuring `data/` exists.

## Versioning, validation, conditional
- Versioning: use helpers from `pkg/versioning/` middleware; avoid non-existent APIs like `WithVersion`/`GetVersion`.
- Validation: use `pkg/validation` functions; do not invent `NewValidator` if it doesn’t exist in the package.
- Conditional/PATCH: implemented via `pkg/conditional`; ETag-based preconditions are wired through middleware.

## Patterns and conventions
- Templates are organized by feature: `templates/{server,client,storage,middleware,reconciliation,authorization}/`.
- Generated routes live in `cmd/server/routes_generated.go`; handlers in `cmd/server/*_handlers_generated.go`; models in `cmd/server/models_generated.go`; OpenAPI in `cmd/server/openapi_generated.go`.
- Examples default to `/resource-plural` endpoints (e.g., `/sensors`).

## Common pitfalls (watch for these)
- Running an old CLI: always use the absolute path to the freshly built binary (`/path/to/fabrica/bin/fabrica`).
- Forgetting `go mod edit -replace …` in test projects when validating local template changes.
- Missing trailing slash in `go run ./cmd/server/`.
- Expecting list responses to include `items` — they’re arrays here.

## When proposing changes
- Show minimal template diff and regenerate a tiny sample to validate.
- Reference the exact files you touched (e.g., `pkg/codegen/templates/middleware/event-bus.go.tmpl`).
- Keep examples and docs in sync; update the matching example README and test script when behavior changes.

## Key references
- Codegen engine: `pkg/codegen/generator.go`
- Templates: `pkg/codegen/templates/**`
- Core types: `pkg/resource`, `pkg/events`, `pkg/reconcile`, `pkg/validation`, `pkg/conditional`, `pkg/versioning`, `pkg/storage`
- Example guides: `examples/**/README.md`, scripts under example folders
