<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# apis.yaml reference

The `apis.yaml` file is the single source of truth for API groups, hub/spoke versions, and imported types. It lives in the **project root**, next to `.fabrica.yaml`, and is created by `fabrica init`.

## File shape

```yaml
groups:
  - name: infra.example.io       # API group
    storageVersion: v1           # Hub (storage) version
    versions:                    # All exposed versions, hub included
      - v1alpha1
      - v1beta1
      - v1
    resources:                   # Populated automatically by `fabrica add resource`
      - Device
    imports:                     # Optional: reuse external Spec/Status types
      - module: github.com/org/pkg
        tag: v1.0.0
        packages:
          - path: api/types
            expose:
              - kind: Device
                specFrom: github.com/org/pkg/api/types.DeviceSpec
                statusFrom: github.com/org/pkg/api/types.DeviceStatus
```

Fields:
- `groups`: list of API groups. Multiple groups are planned; today a single group is supported.
- `name`: fully qualified group name.
- `storageVersion`: hub version used for storage and conversions.
- `versions`: ordered list of all versions (hub + spokes). The hub must be included.
- `resources`: maintained by CLI commands; reflects resources under the hub directory. Accepts either list syntax or configured map syntax.
- `imports`: optional remote type imports exposed to generated APIs.

## Resource generation control

Use list syntax when a resource should use the default path and full generated operation set:

```yaml
groups:
  - name: infra.example.io
    storageVersion: v1
    versions:
      - v1
    resources:
      - Device
      - Rack
```

Use map syntax when a resource needs a custom collection path or a restricted operation set:

```yaml
groups:
  - name: remote-console.openchami.io
    storageVersion: v1
    versions:
      - v1
    resources:
      Console:
        path: /remote-console/consoles
        operations:
          - list
          - get
```

If `path` is omitted, Fabrica uses the default `/<lowercase-resource>s` path. If `operations` is omitted, Fabrica generates the default CRUD plus status surface. When present, `operations` must contain at least one operation; an empty list is rejected. Resource configuration accepts only `path` and `operations`; unknown fields are rejected.

Supported operation values are grouped by the API surface they generate. In the routes below, `<path>` is the configured resource path and `{uid}` identifies one resource.

Read operations:

- `list` generates `GET <path>` to return the resource collection.
- `get` generates `GET <path>/{uid}` to return one resource.

Write operations:

- `create` generates `POST <path>` to create a resource.
- `update` or `put` generates `PUT <path>/{uid}` to replace a resource.
- `patch` generates `PATCH <path>/{uid}` to partially update a resource.
- `delete` generates `DELETE <path>/{uid}` to delete a resource.

Status operations:

- Each of `update-status`, `status-update`, `put-status`, and `updatestatus` generates `PUT <path>/{uid}/status` to replace the resource status.
- Each of `patch-status`, `status-patch`, and `patchstatus` generates `PATCH <path>/{uid}/status` to partially update the resource status.

Operation groups:

- `read` enables `list` and `get`.
- `write` enables `create`, `update`, `patch`, and `delete`.
- `status` enables both status operations.
- `all` or `crud` enables the complete default surface: read, write, and status operations.

Custom paths must be canonical absolute paths without surrounding whitespace, a trailing slash, empty segments, or `.` and `..` segments. Each segment may contain ASCII letters, digits, `.`, `_`, `~`, and `-`. Paths cannot collide with another resource's collection, item, or status routes, or with built-in endpoints such as `/health`, `/openapi.json`, and `/docs`.

Storage can be disabled for generated handlers when the project supplies persistence through create-once persistence hooks. Generated handlers delegate resource access to hooks such as `List<Resource>Resources`, `Get<Resource>Resource`, `Save<Resource>Resource`, and `Delete<Resource>Resource`. When storage generation is enabled, Fabrica creates default hook implementations backed by `internal/storage`; when storage generation is disabled, Fabrica creates stubs that compile but must be implemented by the project.

## Initial workflow

1) `fabrica init <name> [--group <group>] [--versions v1alpha1,v1]`
   - Creates root `apis.yaml` with your group, hub version, and versions (default `example.fabrica.dev` + `v1`).
   - Scaffolds `apis/<group>/<version>/` directories.
2) `fabrica add resource <Name> --version <version>`
   - Writes `apis/<group>/<version>/<name>_types.go` stubs.
   - Adds the resource to `apis.yaml` under `resources`.
3) Edit the generated type stubs.
4) `fabrica generate`
   - Reads `apis.yaml` to discover groups/versions/resources and generates handlers, storage, OpenAPI, and clients.

## Evolving your API

- **Add a new version**: `fabrica add version v1beta2 [--from v1beta1]` copies types from the source spoke into `apis/<group>/v1beta2/` and appends the version to `apis.yaml`.
- **Promote hub**: change `storageVersion` to the new hub, keep the old hub listed in `versions`, and add conversion logic between hub and spokes.
- **Deprecate/remove**: remove the version from `versions` (and associated directories) once clients have migrated.
- **Partial generated operations**: use configured `resources` entries to limit generated routes, handlers, OpenAPI operations, client methods, CLI commands, and starter AuthZ policies for a resource.

## Regeneration behavior

Most generated files are overwritten by `fabrica generate`. A few starter files are intentionally create-once and safe to edit, including starter AuthZ policy files and per-resource persistence hook files. If you change resource paths or operations later, review those create-once files manually because Fabrica will not overwrite local edits.

## Command expectations

- `apis.yaml` must be in the project root.
- `fabrica init` creates it; `fabrica add resource` and `fabrica add version` keep it updated.
- `fabrica generate` enables versioned generation automatically when `apis.yaml` exists; `.fabrica.yaml` no longer carries versioning settings.
