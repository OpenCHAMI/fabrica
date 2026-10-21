<!--
Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica Migrations

Fabrica migrations update existing project source files when generator behavior changes. Run migrations from the project root before regenerating with a newer Fabrica version.

## Available Migrations

| Migration | Command | What it does | When to run |
|-----------|---------|--------------|-------------|
| YAML tags | `fabrica migrate yaml-tags` | Adds missing `yaml` struct tags matching existing `json` tags in resource type files. | Existing services created before resource templates emitted YAML tags. |

## YAML Tags

The YAML tags migration updates files under `apis/<group>/<version>/*_types.go` so YAML data can be unmarshaled into resource structs the same way JSON data can.

For a field like this:

```go
Spec DeviceSpec `json:"spec" validate:"required"`
```

the migration writes:

```go
Spec DeviceSpec `json:"spec" yaml:"spec" validate:"required"`
```

The migration:

- Uses `apis.yaml` to find API groups, versions, and resources.
- Adds `yaml` tags only when a field already has a `json` tag.
- Uses the same tag name and options as the `json` tag, including `omitempty`.
- Preserves existing `yaml` tags unchanged.
- Preserves other tags such as `validate`.
- Skips fields tagged `json:"-"`.
- Formats changed files with `gofmt`.

## How to Migrate

Preview the changes first:

```bash
fabrica migrate yaml-tags --dry-run
```

Apply the migration:

```bash
fabrica migrate yaml-tags
```

Regenerate Fabrica-managed code so generated request models, clients, and storage helpers pick up updated templates:

```bash
fabrica generate
```

For a project outside the current directory, pass `--dir`:

```bash
fabrica migrate yaml-tags --dir /path/to/service --dry-run
fabrica migrate yaml-tags --dir /path/to/service
```

## Troubleshooting

If the migration reports a parse error, fix the Go syntax in the referenced `*_types.go` file and rerun the command. The migration intentionally refuses to rewrite files that cannot be parsed safely.

If the dry-run reports no changes, the resource files already have YAML tags, do not have JSON tags to mirror, or are not listed under the configured API groups and versions in `apis.yaml`.

## Public Service Status Route

Fabrica v0.4.12 generates a public `/service/status` handler and registration function. Newly initialized services call that function automatically. Existing services adopting the generated endpoint must update their project-owned HTTP setup; `fabrica generate` does not rewrite `cmd/server/main.go`.

Use this checklist:

1. Update the service's Fabrica dependency and use the matching Fabrica CLI, then regenerate the service.

   ```bash
   fabrica generate
   ```

2. In `cmd/server/main.go`, add the registration call to the public router group, alongside the existing public routes:

   ```go
   r.Group(func(public chi.Router) {
       public.Get("/health", healthHandler)
       RegisterGeneratedPublicRoutes(public)
       public.Get("/openapi.json", ServeOpenAPISpec)
       public.Get("/docs", ServeSwaggerUI)
       // Preserve other existing public routes.
   })
   ```

   Keep this group outside authentication and authorization middleware, as in the new-service template. Apply those middleware to resource routes separately.

3. If the service already registers `/service/status`, replace that registration when adopting the generated endpoint. Review any custom health logic and response fields before removing its handler. The generated provider initially returns `ok`, and its response contains only `status` and `service`.

4. Review the changes, run focused tests, and build the affected service.

   ```bash
   git diff
   go mod tidy  # If generated imports changed.
   go test ./cmd/server ./internal/...
   go build ./cmd/server
   git diff --check
   ```

5. Start the service and verify an anonymous request, using its configured address:

   ```bash
   curl -i http://localhost:8080/service/status
   ```

   With the default provider, expect HTTP 200 and `{"status":"ok","service":"<service-name>"}`. Repeat this check with authentication enabled to confirm the endpoint remains public.

New services do not need this manual registration step. They get the generated public status route from the start.
