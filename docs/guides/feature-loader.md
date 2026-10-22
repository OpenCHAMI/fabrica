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

## Selective generation and file ownership

A changed feature selection requires full `fabrica generate`. Selective runs such
as `--handlers`, `--storage`, `--openapi`, or `--client` fail before changing output
when the feature selection differs from the last generated composition. With an
unchanged selection, handler/storage generation refreshes feature composition;
OpenAPI-only and client-only generation leave it alone.

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
