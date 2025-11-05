<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

---
layout: post
title: "What the Generated Server Gives You"
description: "A tour of the server you get from Fabrica and how it helps you ship stable APIs."
---

## Context

Shipping a REST API should not mean re‑deciding basic patterns every time. URLs, methods, list shapes, and error handling are easy to do, and easy to do differently. Those small differences raise support costs. Fabrica gives you a full HTTP server that follows the same rules across projects, so you can focus on your domain.

## What you get

Routes are predictable. Each resource gets URLs like /devices and /devices/{uid}, with the right HTTP verbs wired in the generated router (`pkg/codegen/templates/server/routes.go.tmpl`). List responses are plain JSON arrays by design, and the OpenAPI description is generated next to the code (`pkg/codegen/templates/server/openapi.go.tmpl`) so docs match behavior.

Spec and Status are separate on purpose. Spec is what the user wants; Status is what the system reports. The server exposes status‑only endpoints and handlers (`pkg/codegen/templates/server/handlers.go.tmpl`) so controllers can report progress without touching Spec. This pattern is simple, and it prevents many conflicts.

Validation happens in the handler path. Requests are decoded, checked, and validated using helpers in `pkg/validation`. Errors are returned as JSON. Conditional requests and PATCH operations are supported through `pkg/conditional` and `pkg/patch`. You do not have to wire these from scratch.

Storage sits behind a small interface (`pkg/storage/interfaces.go`). The default file backend (`pkg/storage/file_backend.go`) stores JSON and lists UIDs. You can add a database later without changing handlers. The generated server calls storage methods, not filesystem calls.

Events are optional but built in. When enabled, generated handlers publish lifecycle events using `pkg/events/events.go`. You can start with the in‑memory bus (`pkg/events/memory_bus.go`) and swap later. This keeps event logic out of business code.

## Under the hood

The generator reads your resource types and emits handlers that create, read, update, patch, and delete. It keeps generated code in files with a _generated.go suffix under `cmd/server/`. The templates under `pkg/codegen/templates/server/` drive handler layout and route shape. Core behavior—metadata, status conditions, validation, and storage contracts—lives in `pkg/resource`, `pkg/validation`, and `pkg/storage` so it is shared, not copied.

When you add a feature like status subresources or spec version snapshots, the server code grows with it. For example, enabling version snapshots adds routes and handler branches that set `status.version` after spec changes, all emitted by the templates. Your handwritten resource code does not need to change.

## Trade‑offs and limits

Fabrica encodes choices. Lists are arrays, not wrapper objects. Status updates use their own endpoints. This is great for consistency, but it means “creative” deviations are not a target. Fabrica is for RESTful JSON APIs; it is not a GraphQL server or a streaming framework. Keep that scope in mind.

Try it

```bash
fabrica init myapp
fabrica add resource Device
fabrica generate
go run ./cmd/client/ device list -o json
```

## What to watch for in production

Compile the server to deploy it or run locally with `go run ./cmd/server/` (note the trailing slash so all files compile). If you use the file backend, ensure the data directory exists and is writable (defaults to ./data in generated projects). Choose an events bus that fits your environment; the in‑memory bus is for local use.

Related reading

- Guide: `docs/guides/resource-model.md`
- Example: `examples/01-basic-crud/`
