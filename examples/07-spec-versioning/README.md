<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 7: Spec Version History (Opt-in)

**Time to complete:** ~15–20 minutes
**Difficulty:** Beginner → Intermediate
**Prerequisites:** Go 1.23+, Fabrica CLI installed (from a published release)

## What You'll Build

An API with a versioned `Sensor` resource where every spec change creates an immutable snapshot. You can list version history, fetch a specific version, and see the current version directly in the response body as `status.version`. Status updates do not create versions.

## Overview

This example shows how to:
- Opt-in to per-resource spec versioning
- Create snapshots automatically on POST/PUT/PATCH (spec only)
- Keep `status.version` in sync with the latest snapshot
- List, get, and delete version snapshots via dedicated endpoints
- Preserve `status.version` across status-only updates

## Step-by-Step Guide

### Step 1: Initialize a New Project and Add a Versioned Resource

```bash
mkdir -p /tmp/spec-versioning && cd /tmp/spec-versioning
fabrica init myapp
fabrica add resource Sensor --with-versioning
fabrica generate
```

What `--with-versioning` does:
- Adds a marker at the top of `pkg/resources/sensor/sensor.go`:
	`// +fabrica:resource-versioning=enabled`
- Ensures the `SensorStatus` struct includes:
	```go
	// Version is the current spec version identifier (server-managed)
	Version string `json:"version,omitempty"`
	```

### Step 2: Run the Server

```bash
go run ./cmd/server/
```

Important: keep the trailing slash in `./cmd/server/`—there are multiple files to compile.

### Step 3: Use the Generated Client (no cURL required)

Run these from your app directory in a separate terminal while the server runs:

```bash
# Create a sensor (print as JSON to capture values easily)
go run ./cmd/client/ sensor create --spec '{"name":"s1","description":"first"}' -o json

# Save the UID for reuse
UID=$(go run ./cmd/client/ sensor list -o json | jq -r '.[0].metadata.uid')
echo "UID=$UID"

# Check current version in status
go run ./cmd/client/ sensor get "$UID" -o json | jq -r '.status.version'

# Update spec (version changes)
go run ./cmd/client/ sensor update "$UID" --spec '{"description":"second"}' -o json | jq -r '.status.version'

# Patch spec (version changes)
go run ./cmd/client/ sensor patch "$UID" --spec '{"description":"third"}' -o json | jq -r '.status.version'

# List versions
go run ./cmd/client/ sensor versions list "$UID" -o json | jq -r '.[].versionId'

# Get a specific version
VER=$(go run ./cmd/client/ sensor versions list "$UID" -o json | jq -r '.[-1].versionId')
go run ./cmd/client/ sensor versions get "$UID" "$VER" -o json | jq .

# Delete a version (optional)
go run ./cmd/client/ sensor versions delete "$UID" "$VER"
```

Note about status updates: status-only updates don’t create versions and preserve `status.version`. The generated client library supports status updates programmatically (see `pkg/client`), but the CLI focuses on spec operations; if you need to test status updates, add a tiny Go snippet that calls `UpdateSensorStatus` and observe that `status.version` remains unchanged.

## Where Snapshots Live

With the file backend, snapshots are written to:

```
./data/sensors/versions/<uid>/<versionId>.json
```

Each snapshot includes `versionId`, `createdAt`, minimal metadata, and the Spec. Status is not stored in snapshots.

## Troubleshooting

- Server not starting on 8080? Use a different port:
	```bash
	go run ./cmd/server/ serve --port 8081
	```
- Getting empty `status.version`? Ensure your resource file contains the marker and the `Version` field in `Status`:
	```go
	// +fabrica:resource-versioning=enabled
	type SensorStatus struct { /* ... */ Version string `json:"version,omitempty"` }
	```
- Using jq: examples above use `jq` to extract fields; you can also omit `-o json` and view table output.

## Cleanup

```bash
# Delete all sensors using the client
go run ./cmd/client/ sensor list -o json | jq -r '.[].metadata.uid' | xargs -I{} go run ./cmd/client/ sensor delete {}

# Remove data dir
rm -rf ./data
```

## Next Steps

- Read the detailed guide: `docs/guides/spec-versioning.md`
- Add pruning policies or retention controls
- Extend the client CLI to include convenience commands (already generated: list/get/delete versions)
- Consider adding read-by-version (`?version=`) in a future iteration
