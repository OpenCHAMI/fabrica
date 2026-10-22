<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Issue #78: Regenerate-safe feature loader

Status: implemented on the issue #78 feature branch; validation results below.

Assessed on 2026-10-22 against checkout `8ebe1d1`.

Issue: <https://github.com/OpenCHAMI/fabrica/issues/78>

## Viability

**Viable.** Generate the selected feature composition behind a stable startup
interface that user-owned `main.go` calls. Disabled features should be absent
from the composition rather than represented by generated fake types and no-op
implementations. Use explicit, ordered generated calls; a dynamic plugin system,
reflection-based runtime discovery, and a new configuration format are unnecessary.

This completes an existing architectural direction rather than replacing the
generator. The current scaffold already delegates storage, eventing,
reconciliation, and authentication to helper templates. The missing pieces are
regeneration, a stable configuration boundary, and unified feature composition.

Existing customized services need a documented, one-time integration change.
Regeneration alone cannot add loader calls or missing configuration fields to a
user-owned `main.go` without editing that file.

## Evidence from the current implementation

| Source | Current behavior and implication |
| --- | --- |
| `cmd/fabrica/init.go`, `createProjectStructure` | Emits `runtime_helpers_generated.go` and `auth_helpers_generated.go` at initialization. Their generated names do not mean they are refreshed by `generate`. |
| `pkg/codegen/generator.go`, `LoadTemplates` | Loads the metrics helper for regeneration, but does not load runtime or auth helper templates. |
| `pkg/codegen/generator.go`, `GenerateMetrics` | Regenerates metrics; disabled output defines a fake `Metrics` type and no-op methods so unchanged main code compiles. |
| `pkg/codegen/templates/init/main.go.tmpl` | Auth, storage, and reconciliation fields/defaults, some flags, middleware installation, and commands depend on init-time choices. Regenerating helpers alone is insufficient. The file also says `DO NOT EDIT`, contrary to the ownership intended by #78. |
| `cmd/fabrica/generate.go`, `generateRunnerCode` | Copies selected YAML settings into the isolated runner. Reconciliation and AuthZ settings are missing from its configuration model; AuthN combines legacy `features.auth.enabled` and `features.security.authn.enabled`. |
| `internal/config/config.go` | Already validates storage requirements, reconciliation's event dependency, and AuthZ's AuthN dependency. Reuse these semantics. |
| `cmd/fabrica/scaffold_*test.go` and `templates_router_grouping_test.go` | Existing tests provide a starting point for ownership and public/protected route guarantees. Some assertions must move with the implementation boundary. |

The issue's linked commit
[`a0599b0`](https://github.com/OpenCHAMI/fabrica/commit/a0599b0a3985827124bf767078f4146b022226ec)
addresses metrics enablement after init through the current stub mechanism.
The loader should preserve that capability while removing the mechanism for
services that adopt the loader.

## Proposed ownership and interface

Keep `main.go` create-once and user-owned. It retains application command setup,
server configuration, application routes/middleware, the HTTP server, and signal
handling. Correct its header to explain that ownership.

Generate `cmd/server/features_generated.go` with these responsibilities:

- `GeneratedFeatureConfig`, embedded in the scaffold's `Config`, owns feature
  settings that change with generation. Core server fields remain user-owned.
- `applyGeneratedFeatureDefaults` and `registerGeneratedFeatureFlags` supply
  feature defaults and flags before existing Viper binding/unmarshalling.
- `loadGeneratedFeatures(ctx, config)` initializes selected features and returns
  a concrete feature set plus an error. The feature set exposes middleware and
  public/protected route registration, command registration, and cleanup.
- Generated composition references only enabled feature implementations. Generic
  middleware/route collections may be empty; disabled feature-specific types and
  constructors are not emitted merely to satisfy main's references.

Use named functions and explicit dependency order, not package `init()` side
effects. Keep customization points visible in main: global middleware before
route registration, public and protected registration callbacks, and project-owned
storage initialization. Existing create-once reconciler implementations, AuthZ
classifier overrides, OpenAPI extensions, and policy files remain user-owned.

`.fabrica.yaml` selects generated capabilities; service flags/environment values
configure those capabilities at runtime. It is not a runtime reload mechanism.
Preserve existing configuration keys, flag names, defaults, and legacy auth
enablement semantics. Track the current AuthZ environment-driven behavior versus
the documented generator-driven settings explicitly; changing enforcement
precedence/defaults must be a separately reviewed change, not an accidental
effect of moving code.

## Implementation sequence

### 1. Establish one generation model

Extend `GeneratorConfig` and the runner's configuration mapping to carry all
startup inputs: storage/backend/driver, events/bus, reconciliation settings,
metrics, effective AuthN, and AuthZ settings. Introduce a typed feature template
data model shared by init and regeneration to avoid the current
`WithMetrics`/`MetricsEnabled` split.

Reuse existing config validation and normalization before generating anything.
Test legacy auth compatibility and dependency errors. Do not change `apis.yaml`
versioning semantics or introduce new defaults as part of this refactor.

### 2. Generate the loader and migrate the new-service scaffold

Add feature composition/configuration templates and a public
`Generator.GenerateFeatures` method. Load these through `LoadTemplates` and use
the existing formatting and unchanged-file detection machinery. Resolve output
paths from the configured server output directory rather than adding more
hard-coded `cmd/server` paths.

Update init to emit loader-compatible main and feature configuration through the
same model used by generation. Move feature flags/defaults, middleware selection,
and Ent-specific command registration behind the stable interface. Keep main
independent of feature concrete types and init-time feature conditions.

### 3. Compose real implementations and lifecycle handling

Refactor the current runtime/auth/metrics helpers into regenerated feature
implementations. Initialize storage before eventing and reconciliation. Register
all Chi middleware before routes. Apply AuthN then AuthZ only to protected APIs;
keep `/health`, `/openapi.json`, `/docs`, `/service/status`, and enabled metrics
public. Preserve conditional OPTIONS registration.

Unwind already initialized features on startup failure and close them in reverse
order after HTTP shutdown: controller, event bus, storage. Make cleanup safe to
call once through the owning feature set. Retain existing error behavior, including
failure when authentication is enabled but required configuration is absent.

Use the actual controller configuration API when carrying reconciliation settings;
do not claim a setting is honored because it appears in generated logs.

### 4. Integrate generation, selective modes, and cleanup

Invoke `GenerateFeatures` for full generation and handler generation. Storage-only
generation may refresh storage startup composition but must preserve existing
contracts against unrelated routes/models regeneration. OpenAPI-only and
client-only generation should not silently adopt a new server composition.

When a selective run cannot safely update a changed feature and its dependent
artifacts, stop before mutation with an actionable instruction to run full
generation. Test this rule rather than allowing a partially toggled server.

Remove obsolete feature implementations when disabling a feature, including
generated middleware that would otherwise retain imports/dependencies. Delete
only recognized Fabrica-owned files; report conflicts instead of deleting
customized files or directories. Never remove user reconciler implementations,
custom storage, classifier overrides, or policy files.

Update MCP impact predictions and previews for created, rewritten, and removed
artifacts. Continue using the CLI implementation through the existing bridge.

### 5. Provide an explicit compatibility transition

Detect loader adoption from a structural marker in the server scaffold. Generate
the new composition only for adopted services. Preserve the legacy generation
path, including its metrics compatibility helper, for older services during a
documented transition period. Explain that legacy mode retains its existing
init-time limitations; do not silently promise toggle support there.

Provide a migration guide with a before/after main example: embed feature config,
delegate defaults and flags, load the feature set, install middleware/routes,
register commands, and defer cleanup. Users apply that small integration change
once while retaining their application code. No automatic rewrite of customized
main is required for the first release.

Reject uncertain generated-file ownership conflicts with migration instructions.
Remove legacy stubs only after migration when their replacement is emitted.
Compatibility adapters are transitional; loader-based services must not need
disabled-feature stubs.

### 6. Validate and document the contract

Generate real fixture services into temporary directories, compile them, and
exercise HTTP behavior with Go tests. Cover:

| Case | Required result |
| --- | --- |
| Metrics off → on → off | Build succeeds each time; `/metrics` appears/disappears; adopted output has no fake metrics type. |
| AuthN off → on → off | No main edits; protected requests require credentials when enabled; public endpoints remain accessible; missing required auth config fails startup. |
| AuthZ with AuthN | Existing off/shadow/enforce behavior and route classification remain unchanged; invalid feature dependencies are rejected. |
| Events/reconciliation off → on → off | Runtime follows generated selection; failed startup unwinds resources; shutdown stops controller before bus/storage. |
| File, Ent, custom storage | Each supported path builds; Ent commands/imports match the backend; custom initialization remains project-owned. Backend changes do not promise data migration. |
| Repeat generation | No semantic changes or unnecessary rewrites; main and user customization files remain byte-for-byte unchanged. |
| Selective modes and legacy migration | No incomplete feature transitions; existing selective-mode contracts and legacy services still work. |

Update `docs/reference/codegen.md`, `docs/reference/cli.md`, the relevant metrics,
middleware, storage, and reconciliation guides, generated README guidance, and
`pkg/codegen/templates/README.md`. Replace tests that require auth wiring to live
in main with equivalent loader behavior checks; retain ownership assertions.

Run focused tests while implementing, then the repository's required lane:

```bash
GOTOOLCHAIN=go1.26.10 go test ./cmd/fabrica ./internal/config ./pkg/codegen -count=1
GOTOOLCHAIN=go1.26.10 go test ./internal/mcp -count=1
GOTOOLCHAIN=go1.26.10 make build
(cd test/integration && GOTOOLCHAIN=go1.26.10 go mod tidy && GOTOOLCHAIN=go1.26.10 go test -v -timeout 10m)
git diff --check
```

For fixture validation, invoke that checkout-built binary by absolute path with
`generate --fabrica-source /absolute/path/to/fabrica`, run `go mod tidy` when imports
change, and build/test the service. Include at least one Ent fixture with actual
Ent generation. Inspect generated route/OpenAPI/client agreement.

## Delivery and completion criteria

Deliver in three reviewable changes: (1) shared feature model and generator API,
(2) loader/scaffold integration with all supported features and transition
handling, and (3) broader generated-service tests and migration documentation.
Each change must preserve the existing test lane. Do not announce the issue as
complete until the toggling and migration cases above pass.

This is a moderate cross-cutting generator refactor, not a metrics-only template
cleanup. The principal risks are customized-main compatibility, stale generated
imports, configuration drift, and middleware/lifecycle ordering. None is an
architectural blocker if the stable interface and migration boundary are explicit.

## Initial assessment validation

The existing scaffold/router/runner tests passed under Go 1.26.10 using a writable
temporary build cache. `internal/config` also passed its full test suite during
the assessment. An initial full three-package run could not complete successfully
because the sandbox blocked writes to the default Go build/module caches; this
assessment does not establish a passing full generated-service integration lane.
No implementation code was changed for this assessment.

## Implementation outcome

The implementation uses the shared `FeatureConfig` model, regenerated feature
configuration/composition, structural adoption detection, and an explicit legacy
migration path. A generated ownership manifest records fixed feature outputs and
protects edited files. MCP previews report feature writes and removals using the
same preflight checks. Existing services retain metrics compatibility until
migration; adopted services omit disabled startup implementations.

File and Ent service fixtures exercise metrics/AuthN/events/reconciliation
on/off transitions, signed JWT requests, AuthZ off/shadow/enforce, public routes,
failed startup, and repeat cleanup. A file fixture also adopts project-owned custom
storage. Unit tests cover legacy migration, ownership conflicts, selective
changes, and the shared CLI/MCP configuration mapping.

Validation passed under Go 1.26.10: `cmd/fabrica`, `internal/config`,
`pkg/codegen`, `internal/mcp`, `pkg/reconcile`, and `internal/projectfeatures`.
The checkout-built CLI also passed generated file/Ent service tests and builds.
A separate test compiles the actual isolated runner against a legacy generator
API to verify that compatibility does not depend on newer exported symbols.
`git diff --check` is clean.
