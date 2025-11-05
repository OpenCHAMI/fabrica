<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

---
layout: post
title: "Regenerate to Evolve: Adding Features After Release"
description: "How updating Fabrica and regenerating code lets you grow existing APIs without rewrites."
---

APIs live for a long time. They grow as your product grows. The hard part is adding features without breaking what you already shipped. Fabrica is built for this kind of steady change. You keep your resource definitions as the source of truth. The generator keeps the rest in sync.

Here is how it works in practice. Update Fabrica to v0.3.1 or higher. That release added opt‑in spec version history. Take a service you built earlier, add the versioning marker to one resource, and run the generator again. The server, storage helpers, client library, and CLI pick up version endpoints and commands. Your custom code stays where it is.

Under the hood, the CLI flag `--with-versioning` in `cmd/fabrica/add.go` emits a resource marker. The generator tags that resource (`pkg/codegen/generator.go`). Templates extend storage and handlers: snapshots and helpers come from `pkg/codegen/templates/storage/file.go.tmpl`; handlers that set `status.version` live in `pkg/codegen/templates/server/handlers.go.tmpl`. Client methods and CLI subcommands come from `pkg/codegen/templates/client/client.go.tmpl` and `client/cmd.go.tmpl`.

The flow is simple. After regeneration, your API creates snapshots when the spec changes. The current version shows up in the response status. The client gains commands to list versions and fetch a specific one. Snapshots are stored on disk under `./data/<plural>/versions/<uid>/<versionId>.json` when using the file backend.

Trade‑offs and limits

Version snapshots record spec and minimal metadata, not Status. That keeps history focused and small. Retention is not enforced out of the box; you can add pruning later. If your resource changes shape over time, treat version payloads as read‑only history.

Example: configuration you can roll back

Version history shines for configuration. Think of a Sensor with tuning values in its Spec. A user can try a new setting, watch how it behaves, and decide to keep it or go back. Because older versions are never overwritten, there is always a safe place to return to. Rolling back is just “take the spec from an old snapshot and make it the current spec.”

You can do that with the generated client:

```bash
# Pick a prior version (assumes $UID is set)
VER=$(go run ./cmd/client/ sensor versions list "$UID" -o json | jq -r '.[0].versionId')

# Get the snapshot spec
SPEC=$(go run ./cmd/client/ sensor versions get "$UID" "$VER" -o json | jq -c '.spec')

# Apply it as the current spec
go run ./cmd/client/ sensor update "$UID" --spec "$SPEC" -o json
```

Try it

```bash
fabrica init myapp
fabrica add resource Sensor --with-versioning
fabrica generate
go run ./cmd/client/ sensor versions list <uid> -o json
```

What to watch for in production

If you use the file backend, ensure the data directory exists. For SQLite with Ent (in other examples), enable foreign keys with `?_fk=1`. Regeneration is safe because generated files are marked; keep your domain types and logic in non‑generated files.

Related reading

- Guide: `docs/guides/spec-versioning.md`
- Example: `examples/07-spec-versioning/`
