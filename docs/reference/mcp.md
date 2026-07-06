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
- MCP framing via `Content-Length` headers (primary)
- Compatibility input mode for raw JSON envelopes on stdio during handshake/interoperability edge cases
- Initialize protocol negotiation preserves supported client protocol versions and falls back to the latest SDK-supported version for unknown newer versions
- Tool execution constrained to `--workspace`
- Explicit mutating modes: `dry_run` and `execute`; mutating tools default to `dry_run`
- Structured tool error payload with code and remediation
- Agent-ready `recommended_next_calls` in mutating tool results
- Strict argument validation; unknown arguments and wrong types return `invalid_arguments`

## Protocol Negotiation

During `initialize`, Fabrica follows the go-sdk negotiation policy:

- If the client requests a protocol version supported by the bundled SDK, Fabrica responds with that same version.
- If the client requests an unknown newer protocol version, Fabrica falls back to the latest protocol version supported by the bundled SDK.

Currently tested against:

- `2024-11-05` as a preserved supported version
- `2025-11-25` as the current latest supported fallback version

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
- `resources/list`
- `resources/templates/list`
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
- `reconcile`, `reconcile_workers`, `reconcile_requeue`
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

### define_resource_schema

Replace the generated `Spec` and/or `Status` fields for a resource using structured field definitions.

Required inputs:

- `resource_name`

Optional inputs:

- `project_path`, `version`
- `spec_fields`, `status_fields`
- `mode` (`dry_run` or `execute`)

Each field object supports:

- `name` (Go field name)
- `type` (Go type)
- `json_name`
- `required`
- `validation`
- `description`

Dry-run output includes the resource file that would be updated.

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
The response also separates existing generated artifacts into `possible_files` so agents can distinguish files that may already exist from files the selected generation mode intends to write.

### sync_dependencies

Run `go mod tidy` in the target project.

Optional inputs:

- `project_path`
- `mode` (`dry_run` or `execute`)

Dry-run output includes `planned_files` (`go.mod`, `go.sum`) and `planned_steps`.

### build_project

Run `go build` for a Fabrica project.

Optional inputs:

- `project_path`
- `packages` (default `["./..."]`)
- `mode` (`dry_run` or `execute`)

Execute output includes command output and a status.

### test_project

Run `go test` for a Fabrica project.

Optional inputs:

- `project_path`
- `packages` (default `["./..."]`)
- `mode` (`dry_run` or `execute`)

Execute output includes command output and a status.

### smoke_test_api

Check generated API runtime endpoints.

Optional inputs:

- `project_path`
- `base_url` (default `http://localhost:8080`)
- `start_server` (default `false`)
- `server_arguments` (default `["./cmd/server"]`)
- `timeout_seconds` (default `20`)
- `mode` (`dry_run` or `execute`)

When `start_server` is true, the tool runs `go run` temporarily, checks `/health` and `/openapi.json`, then stops the server.

### describe_workflow

Return exact MCP tool-call sequences for common API construction workflows.

Optional inputs:

- `goal`: `new_crud_api`, `add_resource`, or `verify_project`

## Tool Modes

Mutating tools support:

- `dry_run`: return intended impact, no filesystem changes
- `execute`: apply changes

If omitted, `mode` defaults to `dry_run`.

## The Dry-Run Pattern

Fabrica MCP implements a safety-first workflow for all filesystem mutations. To prevent accidental data loss or incorrect project structuring, mutating tools default to `dry_run` mode.

### The Safety Philosophy

Agents should never "blindly" execute mutations. By defaulting to `dry_run`, the server ensures that there is always a conscious transition from *predicting* a change to *applying* it. This prevents common agent errors such as target path drift or unintended file overwrites.

### Understanding the Output

The primary difference between the two modes is the nature of the returned metadata:

- **Dry-Run**: Returns `planned_files` and `planned_steps`. These are predictions of what the server *intends* to do.
- **Execute**: Returns `updated_files`. These are the files that were *actually* modified on disk.

**Example Comparison (`add_resource`):**

*Dry-Run Response (The Plan):*
```json
{
  "status": "dry_run",
  "planned_files": [
    "projects/net-api/apis/net.fabrica.dev/v1/networkswitch_types.go",
    "projects/net-api/apis.yaml"
  ],
  "planned_steps": [
    "Create resource type file for NetworkSwitch",
    "Update apis.yaml with new resource"
  ]
}
```

*Execute Response (The Result):*
```json
{
  "status": "ok",
  "updated_files": [
    "projects/net-api/apis/net.fabrica.dev/v1/networkswitch_types.go",
    "projects/net-api/apis.yaml"
  ]
}
```

### Required Workflow

Agents must follow this sequence for every mutating operation:

1. **Predict**: Invoke the tool (defaults to `dry_run`).
2. **Audit**: Review the `planned_files` list. If any file looks incorrect, adjust arguments and repeat step 1.
3. **Commit**: Invoke the tool again with `mode: "execute"`.

### Decision Matrix

| Scenario | Recommended Mode | Goal |
|----------|------------------|------|
| First attempt at a change | `dry_run` | Verify the target paths and impact |
| Testing new arguments | `dry_run` | See how argument changes affect the plan |
| Final application of change | `execute` | Commit verified changes to the filesystem |

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
- `schema_update_failed`
- `generate_failed`
- `dependency_sync_failed`
- `build_project_failed`
- `test_project_failed`
- `smoke_start_failed`
- `smoke_test_failed`

## Example Flow

Typical agent workflow:

1. Call `inspect_project` to collect context.
2. Call `validate_project` to detect drift or setup gaps.
3. For mutations, call the tool with `mode: dry_run` first.
4. Review `planned_files` and `planned_steps`.
5. Re-run the tool with `mode: execute`.
6. Call `define_resource_schema` to declare structured `Spec`/`Status` fields.
7. Call `generate_code`, `sync_dependencies`, `build_project`, and `test_project`.
8. Optionally call `smoke_test_api` with `start_server: true`.

Agents can also call `describe_workflow` first and follow the returned `workflow` entries directly.

## Practical Examples

### 1. Dry-Run First (Safety)
Call a mutating tool with `mode: "dry_run"` (default) to inspect the impact before applying changes.

**Tool Call:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "add_resource",
    "arguments": {
      "resource_name": "NetworkSwitch",
      "project_path": "projects/net-api"
    }
  }
}
```

**Expected Response:**
```json
{
  "status": "dry_run",
  "planned_files": [
    "projects/net-api/apis/net.fabrica.dev/v1/networkswitch_types.go",
    "projects/net-api/apis.yaml"
  ],
  "planned_steps": [
    "Create resource type file for NetworkSwitch",
    "Update apis.yaml with new resource"
  ]
}
```

**Explanation:** Use this pattern to verify exactly which files will be modified. Only proceed to execution after auditing `planned_files`.

### 2. Execute After Verification
Once the dry-run results are verified, call the same tool with `mode: "execute"` to apply the changes to the filesystem.

**Tool Call:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "add_resource",
    "arguments": {
      "resource_name": "NetworkSwitch",
      "project_path": "projects/net-api",
      "mode": "execute"
    }
  }
}
```

**Expected Response:**
```json
{
  "status": "ok",
  "updated_files": [
    "projects/net-api/apis/net.fabrica.dev/v1/networkswitch_types.go",
    "projects/net-api/apis.yaml"
  ]
}
```

**Explanation:** Explicitly passing `mode: "execute"` signals the server to perform the actual filesystem mutations.

### 3. Complex Schema Definition
Use `define_resource_schema` to move beyond basic structs and define strict validation and JSON naming for API fields.

**Tool Call:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "define_resource_schema",
    "arguments": {
      "resource_name": "NetworkSwitch",
      "spec_fields": [
        {
          "name": "IPAddress",
          "type": "string",
          "json_name": "ipAddress",
          "required": true,
          "validation": "required,ip",
          "description": "Management IP of the switch"
        },
        {
          "name": "PortCount",
          "type": "int",
          "json_name": "portCount",
          "required": false,
          "validation": "min=1,max=48",
          "description": "Number of physical ports"
        }
      ]
    }
  }
}
```

**Expected Response:**
```json
{
  "status": "dry_run",
  "updated_content": "// ... updated Go struct with validation tags ..."
}
```

**Explanation:** Call this to ensure your API contract is strictly typed and validated. The `validation` string is passed directly to the generator's validation logic.

### 4. Guided Workflow Discovery
When unsure of the sequence for a complex task, call `describe_workflow` to get the exact tool chain.

**Tool Call:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "describe_workflow",
    "arguments": {
      "goal": "new_crud_api"
    }
  }
}
```

**Expected Response:**
```json
{
  "workflow": [
    { "step": 1, "tool": "create_service", "description": "Initialize the project" },
    { "step": 2, "tool": "add_resource", "description": "Define your API resources" },
    { "step": 3, "tool": "define_resource_schema", "description": "Set structured fields" },
    { "step": 4, "tool": "generate_code", "description": "Generate handlers and storage" }
  ]
}
```

**Explanation:** Call this to discover the canonical order of operations. Follow the returned `workflow` sequence to avoid configuration gaps.

### 5. Error Remediation
Handle structured errors by reading the `remediation` field to fix the issue without guessing.

**Tool Call (causing error):**
```json
{
  "method": "tools/call",
  "params": {
    "name": "add_resource",
    "arguments": {
      "resource_name": "Device",
      "project_path": "/etc/shadow"
    }
  }
}
```

**Expected Response:**
```json
{
  "isError": true,
  "structuredContent": {
    "status": "error",
    "error": {
      "code": "workspace_violation",
      "message": "Path escapes workspace root",
      "remediation": "Ensure project_path is relative to the --workspace root provided at server startup."
    }
  }
}
```

**Explanation:** When `isError` is true, prioritize the `remediation` string. It provides the direct fix required to resolve the specific error code.


## Related Docs

- [CLI Reference](cli.md)
- [Code Generation Reference](codegen.md)
- [Architecture Reference](architecture.md)
