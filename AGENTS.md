<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Fabrica Agent Guide

Use this file to avoid the mistakes agents most often make in Fabrica: editing generated output, running the wrong binary, missing regeneration, and changing generated API contracts without tests.

## First Decision: What Repository Are You In?

### Working on Fabrica itself

You are in the generator repository when you see packages such as `cmd/fabrica`, `pkg/codegen`, and `pkg/codegen/templates`.

Do this:

1. Change generator code, templates, docs, or tests.
2. Test with Go 1.26.x unless `go.mod` changes.
3. If templates change, prove the generated output changes as intended.

### Working in a Fabrica-generated service

You are in a generated service when the repository has `.fabrica.yaml` and generated files under paths such as `cmd/server/*_generated.go`.

Do this:

1. Change source inputs such as `.fabrica.yaml`, `apis.yaml`, or resource type files.
2. Run the intended `fabrica generate` binary.
3. Run `go mod tidy` if generated imports changed.
4. Build/test the generated service.

Never hand-edit generated files in a generated service.

## Generated File Boundaries

Treat these as generated output in services:

- `cmd/server/*_generated.go`
- `cmd/server/routes_generated.go`
- `cmd/server/models_generated.go`
- `cmd/server/openapi_generated.go`
- `cmd/server/auth_helpers.go`
- `cmd/server/metrics_helpers_generated.go`
- `internal/storage/*_generated.go`
- `internal/storage/ent/**`
- `pkg/client/client_generated.go`
- `pkg/reconcilers/*_generated.go`

In Fabrica itself, the source of truth is usually the matching template under `pkg/codegen/templates/**` or the generator code that feeds that template.

## Regeneration Triggers

Run `fabrica generate` after changing these service inputs:

| Input | Why |
| --- | --- |
| `.fabrica.yaml` | Feature, storage, auth, metrics, and generation settings. |
| `apis.yaml` | API groups, versions, imports, resource membership, resource paths. |
| `apis/<group>/<version>/*_types.go` | Resource envelope, Spec, Status, annotations. |
| `go.mod` / local Fabrica `replace` | Keeps generated code and runtime APIs aligned. |

Do not regenerate for documentation-only changes.

## Local Fabrica Binary Rule

When validating generator changes against a service, use the binary built from the checkout under review.

Typical flow:

```bash
go build -o ./bin/fabrica ./cmd/fabrica
/absolute/path/to/fabrica/bin/fabrica generate --fabrica-source /absolute/path/to/fabrica
```

Do not rely on whatever `fabrica` happens to be first in `PATH`.

## API Contracts Not To Break Casually

- Resource envelopes use `APIVersion`, `Kind`, `Metadata`, `Spec`, and optional `Status`.
- List endpoints return bare JSON arrays, not `{ "items": [...] }`.
- Status is a subresource; spec update endpoints must not mutate status.
- Public service routes such as `/health`, `/openapi.json`, `/docs`, and `/service/status` stay outside auth middleware.
- Generated OpenAPI, routes, clients, and auth policy paths must agree.
- Generated storage must preserve UID, labels, annotations, spec, and status across backends.
- Existing defaults are compatibility contracts. New behavior should be opt-in unless the issue explicitly asks for a breaking change.

## CLI vs MCP

Use the Fabrica CLI as the canonical automation surface for generation,
mutation, and verification.

The MCP server is an adapter over the same operation layer. Use MCP when
an agent runtime exposes it and the task is interactive or tool-native,
but keep CLI parity in mind:

- Prefer CLI commands in reproducible instructions and release validation.
- For mutating MCP operations, use the equivalent dry-run/preview when available.
- After MCP mutations, run the same verification expected for CLI workflows:
  generation, `go mod tidy` when needed, build, tests, and `git diff --check`.
- Do not treat MCP output as a substitute for inspecting generated diffs.

## Testing Expectations

Use the narrowest test that proves the contract:

- Config behavior: test `internal/config`.
- Generator API behavior: test `pkg/codegen` directly.
- Template structure: test template text in `cmd/fabrica` when route grouping or generated shape matters.
- Generated behavior: generate into `t.TempDir()` and inspect or compile the output.

For release-lane work while the module targets Go 1.26.6:

```bash
GOTOOLCHAIN=go1.26.6 go test ./cmd/fabrica ./internal/config ./pkg/codegen -count=1
```

Run `gofmt` with the same selected toolchain when possible.

## Documentation Expectations

- Document user-visible configuration in the relevant guide or reference file.
- Keep examples consistent with current CLI flags and generated paths.
- Prefer specific commands and expected outcomes over prose summaries.
- Keep SPDX headers on source, workflow, and documentation files when the file format supports comments.

## Definition Of Done

- Source inputs changed instead of generated service outputs.
- Changed Go files are formatted.
- Focused tests pass under Go 1.26.x.
- `git diff --check` is clean.
- Generated routes/OpenAPI/client behavior stays internally consistent.
- Documentation is updated for user-facing config or behavior.
- No commits or pushes unless explicitly requested.

## Quick References

- Generator core: `pkg/codegen/generator.go`
- Templates: `pkg/codegen/templates/**`
- CLI: `cmd/fabrica/**`
- API config reference: `docs/apis-yaml.md`
- Codegen reference: `docs/reference/codegen.md`
- Storage guide: `docs/guides/storage.md`
