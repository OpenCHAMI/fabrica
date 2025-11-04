<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Spec Version History (Opt-in)

> Record immutable snapshots of a resource's Spec on create/update/patch, list version history, and retrieve specific versions. Status is never versioned. The current spec version is surfaced as `status.version`.

## What you get

- Per-resource opt-in snapshots of Spec (+ minimal metadata)
- Endpoints to list/get/delete snapshots:
  - `GET /<plural>/{uid}/versions`
  - `GET /<plural>/{uid}/versions/{versionId}`
  - `DELETE /<plural>/{uid}/versions/{versionId}`
- Server-managed `status.version` updated on spec changes
- No snapshots on status updates

Snapshot schema:

```json
{
  "versionId": "20251104161025-1762273000000000000",
  "createdAt": "2025-11-04T16:10:25Z",
  "uid": "sen-1234abcd",
  "name": "sensor-1",
  "labels": {"env":"dev"},
  "annotations": {"owner":"team-a"},
  "spec": { }
}
```

Version IDs are time-sortable strings: `YYYYMMDDHHMMSS-nanots`.

## Enable for a resource

1) When adding a resource, pass the flag:

```bash
fabrica add resource Sensor --with-versioning
```

This inserts a marker at the top of the resource file:

```go
// +fabrica:resource-versioning=enabled
```

2) Ensure your Status struct includes a `Version` field (the scaffolder adds this when `--with-versioning` is used):

```go
type SensorStatus struct {
    Phase   string `json:"phase,omitempty"`
    Message string `json:"message,omitempty"`
    Ready   bool   `json:"ready"`
    // Version is the current spec version identifier (server-managed)
    Version string `json:"version,omitempty"`
}
```

3) Generate code:

```bash
fabrica generate
```

The generated handlers will:
- Create a new snapshot on POST/PUT/PATCH (spec changes)
- Update `status.version` to the new snapshot ID and persist it
- Preserve `status.version` across status-only updates

## Try it

Assuming a generated server running at http://localhost:8080 and a versioned `Sensor` resource:

```bash
# Create Sensor
curl -s -H 'Content-Type: application/json' \
  -d '{"name":"s1","description":"first"}' \
  http://localhost:8080/sensors | jq .

# Observe status.version in the response body

# Update Spec
curl -s -X PUT -H 'Content-Type: application/json' \
  -d '{"description":"second"}' \
  http://localhost:8080/sensors/<uid> | jq .status.version

# Patch Spec
curl -s -X PATCH -H 'Content-Type: application/merge-patch+json' \
  -d '{"description":"third"}' \
  http://localhost:8080/sensors/<uid> | jq .status.version

# Status update (does NOT change version)
curl -s -X PUT -H 'Content-Type: application/json' \
  -d '{"ready":true}' \
  http://localhost:8080/sensors/<uid>/status | jq .status.version

# List versions
curl -s http://localhost:8080/sensors/<uid>/versions | jq .

# Get a specific version
curl -s http://localhost:8080/sensors/<uid>/versions/<versionId> | jq .

# Delete a specific version
curl -s -X DELETE http://localhost:8080/sensors/<uid>/versions/<versionId>
```

## Implementation details

- Storage: file backend writes snapshots to `./data/<plural>/versions/<uid>/<versionId>.json`
- Handlers set `status.version` on spec mutations and resave the resource
- `Latest<Kind>VersionID` helper finds the current version when needed
- OpenAPI paths are generated for version operations

## Caveats and next steps

- Read-by-version (`?version=`) and default-version pinning are not enabled yet
- Snapshots only include Spec + minimal metadata (no Status)
- Consider pruning policies if version growth is a concern

For a runnable walk-through, see the example at `examples/07-spec-versioning/`.
