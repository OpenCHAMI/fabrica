<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

---
layout: post
title: "Pluggable storage in Fabrica: files to databases"
description: "How the storage contract lets you swap file and database backends without changing your API, with a look at the FRU example."
---

## Context

Fabrica generates REST services from Kubernetes‑style resources. The shape is always the same: APIVersion, Kind, Metadata, Spec, and Status. Handlers, routes, models, and the client are generated from templates. What you store those resources in is up to you. The storage layer is pluggable, so you can start with files and move to a database later without rewriting your API.

The key idea is separation. The handlers talk to a storage interface, not to disks or SQL directly. You can see that in `pkg/codegen/templates/server/handlers.go.tmpl`. The code calls storage helpers with typed resources and returns results to the client. The same handler code compiles whether you pick the file backend or the Ent (database) backend.

## What you get

You get a consistent API surface and a storage implementation that matches it. The file backend writes JSON objects to disk. It is simple, great for demos and local dev, and easy to inspect. The database backend uses Ent, a type‑safe ORM for Go. It brings relationships, indexes, and migrations. Both satisfy the same storage contract declared in `pkg/storage/interfaces.go`.

Nothing above storage needs to change. Validation and conditional requests keep working. Those come from middleware in the server templates. Events still publish lifecycle and condition changes (see `pkg/events/events.go`). Reconciliation, when enabled, still processes work (see `pkg/reconcile/controller.go`). Even the code generation knobs are the same. The generator reads your resources and features and emits the right code (see `pkg/codegen/generator.go` and the CLI hook in `cmd/fabrica/add.go`).

## How it works under the hood

The file backend lives in `pkg/codegen/templates/storage/file.go.tmpl`. It organizes data by resource kind and UID under a data directory. Create, update, list, and delete are plain file operations plus JSON encoding and decoding. If you enable spec version history for a resource, version snapshots are also files on disk in a versions folder.

The database backend uses Ent. The adapter is in `pkg/codegen/templates/storage/ent.go.tmpl`. The schemas for annotations, labels, and resources are in `pkg/codegen/templates/ent/schema/*.go.tmpl`. Generated servers open a database connection, run schema creation in development, and set the storage adapter. Handlers don’t know or care which backend you chose, because they call through the same interfaces.

The handlers template, `pkg/codegen/templates/server/handlers.go.tmpl`, wires requests to storage calls. It also makes the Spec vs. Status split explicit. Spec is what a user sets. Status is managed by the system. That separation is important because it means status updates do not mix with spec persistence logic. It also makes versioning possible to implement cleanly.

## Trade‑offs and limits

Files are simple and fast to get started with. They are also easy to back up and read during debugging. The trade‑off is querying and constraints. If you need strong relationships, transactions, or complex queries, a database is a better fit. Ent gives you typed queries, migration helpers, and schema‑as‑code.

Databases add operational work. You will think about migrations, connection strings, and indexes. The upside is predictable behavior at scale. The storage choice does not change your API, so you can migrate later without breaking clients.

Events and reconciliation do not depend on storage. They depend on the resource model. Event types and helpers are in `pkg/events/events.go`. The reconciliation controller is in `pkg/reconcile/controller.go`. You can enable or disable those features without touching storage code.

## Try it

Here is a tiny flow that shows the storage plug‑in with the generated client. It uses file storage so you can try it anywhere. Run the server in the background and call it from the client.

```bash
fabrica init store-demo --module github.com/you/store-demo --storage-type file
fabrica add resource Widget
fabrica generate && go run ./cmd/server/ serve --data-dir ./data &
go run ./cmd/client/ widget list --output json
```

This is the same flow you would use for a database backend. Change the init flags to `--storage-type ent --db sqlite` and start the server with `--database-url "file:data/app.db?_fk=1"`. The SQLite foreign key pragma (`?_fk=1`) is required so relationships enforce correctly.

## What to watch for in production

Think about where your data lives and how you back it up. The file backend writes under a data directory (by default `./data`). Make sure that directory exists and is writable. If you run containers, mount a volume. If you change the path, keep it consistent across restarts.

For Ent, pin and run migrations outside the server for production. The init template sets up schema creation that is great for development, but real deployments benefit from controlled migration steps. Watch connection strings and options. For SQLite, include `?_fk=1` so foreign keys enforce. For Postgres and MySQL, make sure drivers are on your path and your URLs match your environment.

Events are optional. The in‑memory bus is perfect for local development, but a real system may want a durable bus. The event helpers live in `pkg/events/events.go`. You can swap the bus implementation without changing handlers. Reconciliation is optional too. It depends on events and the resource model, not on storage internals.

Concurrency and conflicts are handled above storage. Conditional requests use ETags so you can protect writes. That logic is generated into middleware and handlers, not buried in the storage adapter. The separation of layers keeps storage simple and the API stable.

## Related reading

If you want to see a database in action, read the FRU service example at `examples/03-fru-service/README.md`. It uses Ent with SQLite and shows a richer resource model for hardware inventory.

For a deeper dive on database storage, see `docs/guides/storage-ent.md`. If you are staying on files for now, the backend implementation in `pkg/codegen/templates/storage/file.go.tmpl` is short and worth a read. You can also scan `pkg/codegen/templates/server/handlers.go.tmpl` to see the Spec vs. Status split and how requests map to storage calls. Finally, `pkg/codegen/generator.go` ties the feature flags and resource metadata to the templates, and `cmd/fabrica/add.go` shows how new resources are wired into the code generator.

The bottom line: pick the backend that fits today. You can change it later with a config edit and a regeneration step. Your routes and clients don’t need to know the difference.
