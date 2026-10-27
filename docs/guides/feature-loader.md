<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Regenerate-safe startup features

New Fabrica services use a generated feature loader. Edit `.fabrica.yaml`, run
full generation, update dependencies, and rebuild to change the selected features:

```bash
fabrica generate
go mod tidy
go test ./...
go build ./cmd/server
```

`main.go` is user-owned and is never rewritten by `generate`. The generated loader
owns feature configuration fields/defaults/flags, storage startup, eventing and
reconciliation startup, metrics, authentication, protected middleware, and
storage-specific commands. Disabled startup features are omitted; they do not
require fake types or no-op implementations.

You can split `Config` and startup code into separate application Go files under
`cmd/server`. The selected build must include both the embedded
`GeneratedFeatureConfig` and the call to `loadGeneratedFeatures`; tests and
generated helpers do not establish loader adoption. For custom build tags, pass
the same `GOFLAGS` environment to generation and the service build, for example:

```bash
GOFLAGS='-tags=prod,feature' /path/to/fabrica generate --fabrica-source /path/to/fabrica-source
GOFLAGS='-tags=prod,feature' go build ./cmd/server
```

Generation honors `-tags` in this environment and excludes inactive variants.
Tags supplied only to a separate `go build -tags=...` invocation are not inherited.

This is build-time composition, not runtime reloading of `.fabrica.yaml`. Service
flags, environment variables, and service YAML continue configuring the selected
capabilities with the existing names and defaults. Embedded feature configuration
uses `mapstructure:",squash"` to preserve flat keys such as `data_dir`,
`database_url`, and `auth_enabled`.

## Changing features

For example, enable metrics with:

```yaml
features:
  metrics:
    enabled: true
    provider: prometheus
```

Run the commands above. `/metrics` becomes available; disabling metrics and
regenerating removes its route and implementation.

Validation uses `features.validation.enabled` and `features.validation.mode` at
generation time. Strict is the default; warn logs optional Spec/Status tag and
custom-hook failures and continues, and disabled skips these optional checks.
`enabled: false` overrides any valid mode; `mode: disabled` also disables optional
checks even with `enabled: true`. Invalid modes reject configuration in all cases.
Regenerate, rebuild, and restart after changing the policy. Runtime validation
settings do not override generated policy. Required decoding, identity/envelope,
patch, authorization, ETag, immutable-field, and backend checks remain mandatory.
See the [validation policy table](validation.md#generated-http-validation-policy)
for the full boundary and examples.
Generated creates reject omitted/null required envelopes in all modes. Explicit
empty objects are distinct from absence; nested validation remains mode-dependent.
Revision creation/ensure requires `spec` and takes identity from the URL.

Authentication is generated when either `features.security.authn.enabled` or the
legacy `features.auth.enabled` is true. To disable it, set both to false. Enabled
authentication still requires `TOKENSMITH_JWKS_URL` at startup. AuthZ retains the
existing `TOKENSMITH_AUTHZ_MODE` selection (`off`, `shadow`, `enforce`; default
`off`) and Casbin environment settings. This loader does not change AuthZ
configuration precedence or enforcement defaults. The generator carries AuthZ
settings for validation and change detection; it does not introduce new runtime
enforcement semantics for those settings.

When adding authentication to a service that has no TokenSmith dependency, select
the supported version before tidying:

```bash
go get github.com/openchami/tokensmith@v0.3.0
go mod tidy
```

Reconciliation requires events and uses the configured worker count (default 5).
`features.reconciliation.requeue_delay` selects the generated periodic interval in
minutes (default 5). The service's existing `reconcile_workers` runtime key can
configure the generated controller's concurrency. Cleanup stops the controller,
then the event bus, then storage. Startup failures unwind initialized features.

Custom storage remains project-owned: initialize it in main before loading
features. Switching storage backends is not a data migration. Existing checks
reject stale Fabrica storage artifacts when switching to custom storage; preserve
or migrate data and remove/replace those artifacts yourself.

Switching an adopted service from Ent to file storage also requires an explicit
migration. Full generation fails before changing output while Ent backend files
remain. This also applies when a legacy Ent service adopts the loader and selects
file storage in the same generation; until the first manifest exists, any
remaining Ent file under `internal/storage/` identifies the old backend. Back up
or migrate the database and preserve any customizations, then move the paths
listed in the error outside the service: `internal/storage/ent/`,
`ent_adapter.go`, any `ent_adapter_*.go`, `ent_queries_generated.go`,
`ent_transactions_generated.go`, `ent_revisions_generated.go` when present, and
`generate.go` under `internal/storage/`,
plus `cmd/server/export.go` and `cmd/server/import.go`. Leave
`internal/storage/storage_generated.go` in place for regeneration. Run full
`fabrica generate`, `go mod tidy`, `go test ./...`, and `go build ./cmd/server`.
The loader does not delete or take ownership of these older backend files.

## Selective generation and file ownership

A changed feature selection requires full `fabrica generate`. Selective runs such
as `--handlers`, `--storage`, `--openapi`, or `--client` fail before changing output
when the feature selection differs from the last generated composition. With an
unchanged selection, handler/storage generation refreshes feature composition;
OpenAPI-only and client-only generation leave it alone.

For Go integrations, configure the complete selection with `Generator.SetFeatures`
and call `Generator.GenerateAll`. Server generation checks feature ownership
before writing any outputs and refreshes the loader, enabled implementations,
and manifest. Legacy services retain their existing middleware and metrics helpers.

Commit `cmd/server/features_manifest_generated.json` alongside generated files.
It records the feature selection and hashes of loader-owned outputs. It is not
runtime configuration. Regeneration detects edited or untracked feature files
before replacing or removing them. Preserve the customization in user-owned code,
move the conflicting file aside, and generate again. Do not resolve the conflict
by editing the manifest hashes.

The manifest covers loader, runtime/auth helpers, metrics, and generated feature
middleware. It does not take ownership of user storage, reconciler implementations,
AuthZ classifier overrides, OpenAPI extensions, or policies. Previously generated
reconciler support and the default AuthZ classifier remain available to their
user-owned implementations even after startup is disabled; their imports may
therefore keep dependencies in `go.mod`.

MCP dry runs use the same feature preflight and include `planned_removals` for
obsolete feature outputs. The CLI remains the canonical generation interface.

## Recovering interrupted feature generation

Full generation saves a versioned recovery journal before changing loader-owned
outputs. The journal captures the feature selection, old/target hashes (including
absence), the exact target manifest, and the generated target bytes. The target
bytes are staged inside the journal, so recovery does not depend on temporary
files or rerendering templates from a newer Fabrica binary. Every output and the
manifest are replaced with a flushed temporary file and a rename; the manifest
is committed after the feature outputs match their recorded targets. Journals
are limited to 16 MiB; oversized staged output rejects before live writes.

After an interrupted run, use the same full command:

```bash
fabrica generate
# When validating a local checkout:
/absolute/path/to/fabrica/bin/fabrica generate --fabrica-source /absolute/path/to/fabrica
```

Full generation first finishes the captured transaction, then applies the newly
requested `.fabrica.yaml` selection. Changing YAML while recovery is pending does
not change the captured target. A new selection can still fail its own normal
preflight after the previous transaction has been recovered. Selective commands
such as `--handlers` or `--client` reject pending recovery before any writes and
direct you to full generation.

Recovery accepts only each file's captured old or target state. An unrecognized
edit, symlink, special file, or corrupt journal stops recovery and retains the
evidence. Back up custom edits outside the service, then restore an exact recorded
old or target version for the conflicting path before retrying. The journal keeps
the target bytes in its base64-encoded `Data` fields. Moving a conflicting file
aside is sufficient only when absence is one of its recorded states. Do not edit
manifest hashes or delete the journal to bypass recovery.

Recovery checks journal paths against the feature outputs of the generator
performing recovery, never against paths read from the journal. The `fabrica`
CLI recovers in its own preflight, before it builds the project's Fabrica
module or `--fabrica-source` checkout, so finish an interrupted run with a
`fabrica` binary from the same Fabrica version that wrote the journal, then
upgrade. A binary with a different set of feature outputs reports the version
mismatch and keeps the journal.

These local files are reserved generation metadata:

- `cmd/server/.fabrica-features-transaction.json`: pending recovery; removed after commit.
- `cmd/server/.fabrica-features.lock`: persistent lock file; do not remove while generators run.
- `*.fabrica-tmp` beside owned outputs: temporary replacement files, cleaned during retry.

New scaffolds ignore these paths in Git. Existing services should add the same
basename patterns to their `.gitignore`; continue committing the public
`features_manifest_generated.json` with generated outputs. Preserve pending
recovery metadata when moving or backing up an interrupted checkout.

Mutation and recovery use an OS file lock shared by generators in the same
service. Process termination releases the lock. The transaction covers only
loader-owned feature files and their ownership manifest; it does not make all
server/client generation or storage migrations atomic. Legacy services retain
their existing overwrite behavior. Avoid editing owned files during generation.

Files are synced before rename, and directory metadata is synced on Unix where
supported. Process-interruption recovery is covered by tests. Power-loss
durability depends on filesystem/OS guarantees, including support for advisory
locking, atomic replacement, and reliable sync. Windows flushes staged files but
has no portable directory-sync guarantee through this implementation. Platforms
without supported file locking reject adopted feature generation explicitly.

MCP `generate_code` dry runs remain read-only. When a transaction is pending,
they return `status: recovery_required`, its captured configuration, affected
`planned_files`/`planned_removals`, and the full-generation next action. Conflicting
edits are reported in `recovery_conflict`; neither the files nor recovery metadata
are repaired by a preview. Go callers can use `InspectFeatureRecovery` for the
same read-only inspection and `PrepareFeatureGeneration(true)` for mutating full
preflight. `GenerateAll` and `GenerateFeatures` perform that preflight themselves.

## Migrating an existing service

Older main scaffolds continue using the legacy generation path, including metrics
compatibility helpers. Their other init-time limitations remain until migrated.
Migration is explicit because `main.go` may contain application customizations.
Use a current checkout-built Fabrica binary when validating the migration.

1. Preserve your current main and helper customizations in version control.
2. Keep core server fields in `Config`, remove the old feature-specific fields,
   and embed the generated feature configuration:

   ```go
   type Config struct {
       // Keep Port, Host, timeouts, Debug, and application fields here.
       GeneratedFeatureConfig `mapstructure:",squash"`
   }
   ```

3. In `DefaultConfig`, build the core config, call
   `applyGeneratedFeatureDefaults(config)`, and return it. Remove the old feature
   defaults from the struct literal. Replace feature flag definitions with
   `registerGeneratedFeatureFlags(serveCmd.Flags())` before Viper binding. Replace
   Ent export/import command registration with
   `registerGeneratedFeatureCommands(rootCmd)`.
4. Replace the individual storage/event/auth/metrics initialization calls with:

   ```go
   features, err := loadGeneratedFeatures(cmd.Context(), config)
   if err != nil {
       return err
   }
   defer features.Close()
   ```

5. Call `features.Use(r)` before registering any routes. Keep public and protected
   Chi groups and application routes. Replace feature-specific route/middleware
   branches with:

   ```go
   r.Group(func(public chi.Router) {
       public.Get("/health", healthHandler)
       RegisterGeneratedPublicRoutes(public)
       public.Get("/openapi.json", ServeOpenAPISpec)
       public.Get("/docs", ServeSwaggerUI)
       features.RegisterPublicRoutes(public)
       // Register application public routes here.
   })
   r.Group(func(protected chi.Router) {
       features.UseProtected(protected)
       RegisterGeneratedRoutes(protected)
       // Register application protected routes here.
   })
   ```

6. Move existing `runtime_helpers_generated.go`, `auth_helpers_generated.go`,
   `metrics_helpers_generated.go`, and `metrics_generated.go` out of `cmd/server`
   if present. Also move the generated validation/conditional/versioning/event-bus
   middleware files out of `internal/middleware`. Preserve any customizations in
   project-owned code. These files predate the ownership manifest, so the loader
   deliberately refuses to overwrite or delete them without that step. Do not
   remove entire middleware directories or user files.
7. Run full generation, tidy, build, and tests. Confirm that public endpoints are
   accessible and protected APIs retain the intended authentication/authorization.
   Commit the new generated files and manifest with the one-time main change.

Keep application-specific storage hooks, reconcilers, policies, and classifiers
in their user-owned files. Their existing create-once behavior is unchanged.

## Acceptance checks in the Fabrica checkout

Use Go 1.26.10 to run the independent feature and backend/migration scenarios:

```sh
GOTOOLCHAIN=go1.26.10 go test ./cmd/fabrica -run 'TestFeatureLoader(IndependentAcceptance|BackendDatasetTransitions|LegacyExplicitAdoption)$' -count=1
```

These tests build the CLI from the checkout and invoke it by absolute path with
`--fabrica-source`. They compare supported `init` flags and first-generation YAML
settings with regenerated output, then exercise HTTP behavior after rebuilding.
Metrics, events/reconciliation, conditional requests, and AuthN are changed in
separate sequences. Checks include metric observation/route removal, SHA-256/MD5
ETags over actual payloads, 304/412 responses, signed-token/public-route behavior,
worker concurrency, periodic requeue results, and real startup/shutdown cleanup.

The backend scenario preserves file data, explicitly transfers a complete resource
envelope into SQLite/Ent, preserves the old backend source/database outside the
service, and transfers into a new file destination. Backend selection itself does
not copy data. The original datasets and application source must remain intact.
The pre-existing empty Ent collection response shape is tracked separately and
is not changed or asserted as corrected by these loader tests.

The live legacy scenario uses matching historical main/runtime-helper fixtures.
It confirms legacy metrics helpers still work, while changing event/AuthN YAML
alone retains the documented init-time limitation. After the explicit migration,
the same application route observes event startup and subsequent removal without
further main edits. The separately covered AuthN/AuthZ tests retain JWKS startup
requirements and `TOKENSMITH_AUTHZ_MODE` off/shadow/enforce behavior.
