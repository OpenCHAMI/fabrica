---
layout: post
title: "Meet the Generated Client"
description: "How the CLI and Go library help you use your API the same way every time."
author: "Alex Lovell-Troy"
---

APIs are easier to use when the client is predictable. You should not have to remember custom flags or one‑off scripts. Fabrica generates two clients that match your server: a CLI and a Go library. They share patterns and types, so your shell steps and your code read the same way.

## What you get

The CLI is generated from `pkg/codegen/templates/client/cmd.go.tmpl`. Commands are grouped by resource: list, get, create, update, patch, delete. Output can be tables or JSON, which makes it simple to script with tools like jq. Version‑specific commands appear only when a resource supports them, so the CLI is always in sync with your API.

The Go client library comes from `pkg/codegen/templates/client/client.go.tmpl`. Method names are predictable and typed: `Get<Device>`, `Create<Device>`, `Update<Device>`, `Patch<Device>`, and status‑only methods like `Update<Device>Status`. The client handles headers, content types, and patch formats for you. When API version headers are used, the client sets them.

## Under the hood

The CLI creates a typed client with `client.NewClient` and calls methods that mirror your resources. The methods issue HTTP requests to the server’s generated routes (`pkg/codegen/templates/server/routes.go.tmpl`) and unmarshal responses into your types from `pkg/resources/...`. Status operations go to `/status` endpoints to avoid spec conflicts.

When you enable spec versioning on a resource, the generator adds version helpers and subcommands. The Go client gets `List<Resource>Versions`, `Get<Resource>Version`, and `Delete<Resource>Version`, and the CLI nests them under `<resource> versions ...`. These come from the same templates, so you do not maintain them by hand.

## Trade‑offs and limits

The generated client is opinionated. It follows the server’s routes and types exactly. That keeps things consistent, but it is not a generic HTTP tool. If you need custom flows or experiments, you can wrap the client in your own package.

## Try it

```bash
fabrica init myapp
fabrica add resource Device
fabrica generate
go run ./cmd/client/ device create --spec '{"name":"dev-01","description":"demo"}' -o json
```

## What to watch for in production

Prefer `-o json` when you need to script. Use the status methods when workers or controllers update state, so spec and status do not clash. If your API adds versioned endpoints, re‑generate so the client gains matching commands and types.

## Related reading

- Guide: `docs/guides/quickstart.md`
- Example: `examples/01-basic-crud/`
