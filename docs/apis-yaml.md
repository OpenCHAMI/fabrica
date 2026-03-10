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
- `resources`: maintained by CLI commands; reflects resources under the hub directory.
- `imports`: optional remote type imports exposed to generated APIs.

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
- **Partial features per version**: encode per-version status subresource or flags in future extensions of `apis.yaml`; avoid putting per-version knobs in `.fabrica.yaml`.

## Command expectations

- `apis.yaml` must be in the project root.
- `fabrica init` creates it; `fabrica add resource` and `fabrica add version` keep it updated.
- `fabrica generate` enables versioned generation automatically when `apis.yaml` exists; `.fabrica.yaml` no longer carries versioning settings.
