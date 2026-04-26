<!--
Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Fabrica MCP Mode Reference

> Detailed reference for running Fabrica as a local Model Context Protocol (MCP) server.

## Overview

`fabrica mcp` starts a local MCP server over stdio so agents and editor integrations can inspect and modify Fabrica projects in a constrained workspace.

Key characteristics:

- JSON-RPC 2.0 message envelope
- MCP framing via `Content-Length` headers
- Tool execution constrained to `--workspace`
- Explicit mutating modes: `dry_run` and `execute`
- Structured tool error payload with code and remediation

## Start The Server

```bash
fabrica mcp --workspace /path/to/workspace
```

### CLI Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--workspace` | string | `.` | Root directory allowed for all MCP operations |

## Supported MCP Methods

- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

## Available Tools

### inspect_project

Inspect a project and return key metadata:

- project info (`project_name`, `module`)
- API group and versions
- storage settings and selected features
- discovered resources

Inputs:

- `project_path` (optional, default `.`)

### validate_project

Validate project structure and config consistency.

Inputs:

- `project_path` (optional, default `.`)

Returns:

- `status`: `ok`, `warning`, or `error`
- `issues`: machine-readable issue entries with `severity`, `code`, and `message`

Current checks include:

- missing `.fabrica.yaml` or `apis.yaml`
- invalid config content
- version directory drift (`apis/<group>/<version>`)
- resource list drift between `apis.yaml` and storage version type files
- stale generated artifacts (type files newer than generated routes)

### create_service

Create a new Fabrica project.

Required inputs:

- `project_name`

Optional inputs:

- `target_dir`, `module`, `description`
- `group`, `storage_version`, `versions`
- `auth`, `metrics`, `events`, `events_bus`
- `storage_type`, `db`, `validation_mode`
- `mode` (`dry_run` or `execute`)

Dry-run output includes `planned_files` and `planned_steps`.

### add_resource

Add a resource type file and update `apis.yaml`.

Required inputs:

- `resource_name`

Optional inputs:

- `project_path`, `version`
- `with_validation`, `with_status`, `with_versioning`
- `package`, `force`
- `mode` (`dry_run` or `execute`)

Dry-run output includes `planned_files` and `planned_steps`.

### add_version

Add a new API version and optionally copy type files from another version.

Required inputs:

- `new_version`

Optional inputs:

- `project_path`, `from`, `force`
- `mode` (`dry_run` or `execute`)

Dry-run output includes `planned_files` and `planned_steps`.

### generate_code

Run code generation for selected artifact groups.

Optional inputs:

- `project_path`
- `artifacts` (`handlers`, `storage`, `client`, `openapi`)
- `force`, `debug`, `fabrica_source`
- `mode` (`dry_run` or `execute`)

Dry-run output includes predicted `planned_files` and `planned_steps`.

### sync_dependencies

Run `go mod tidy` in the target project.

Optional inputs:

- `project_path`
- `mode` (`dry_run` or `execute`)

Dry-run output includes `planned_files` (`go.mod`, `go.sum`) and `planned_steps`.

## Tool Modes

Mutating tools support:

- `dry_run`: return intended impact, no filesystem changes
- `execute`: apply changes

If omitted, `mode` defaults to `execute`.

## Workspace Safety

All tool paths are resolved relative to `--workspace`.

If an input path escapes the workspace root, the tool returns a structured error with `code` set to `workspace_violation`.

## Error Contract

Tool failures are returned in `tools/call` results with:

- `isError: true`
- `structuredContent.status: error`
- `structuredContent.error.code`
- `structuredContent.error.message`
- `structuredContent.error.remediation`

Common error codes:

- `invalid_arguments`
- `workspace_violation`
- `invalid_workspace`
- `missing_config`
- `missing_apis_config`
- `invalid_config`
- `invalid_apis_config`
- `resource_discovery_failed`
- `create_service_failed`
- `add_resource_failed`
- `add_version_failed`
- `generate_failed`
- `dependency_sync_failed`

## Example Flow

Typical agent workflow:

1. Call `inspect_project` to collect context.
2. Call `validate_project` to detect drift or setup gaps.
3. For mutations, call the tool with `mode: dry_run` first.
4. Review `planned_files` and `planned_steps`.
5. Re-run the tool with `mode: execute`.
6. Call `generate_code` and `sync_dependencies` as needed.

## Related Docs

- [CLI Reference](cli.md)
- [Code Generation Reference](codegen.md)
- [Architecture Reference](architecture.md)
