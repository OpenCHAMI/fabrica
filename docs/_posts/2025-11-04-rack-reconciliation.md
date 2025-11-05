---
layout: post
title: "Reconciliation: Let the System Do the Work"
description: "A gentle look at controllers that react to change and keep resources in a good state."
author: "Alex Lovell-Troy"
---

Some jobs are better done by the system than by users clicking through steps. Reconciliation makes this possible. You declare what you want in Spec, and a controller reads that intent and works until the resource reaches a steady state. When things drift, the controller nudges them back.

In Fabrica, reconciliation is a first‑class pattern. The generated server publishes events when resources change, and the reconciliation controller listens and enqueues work. Your code reacts by reading the resource, taking actions, and updating Status. See `pkg/events/events.go` for event helpers and `pkg/reconcile/controller.go` for the controller.

## What you get

You get a clean split between user intent and system state. Users set Spec. Controllers update Status. The handlers template (`pkg/codegen/templates/server/handlers.go.tmpl`) keeps those paths separate so your controller never fights with user writes.

You also get the plumbing: event publishing from handlers, an in‑memory event bus for local runs, a controller with a work queue, and hooks to register your reconcilers. The CLI wires features into generation (see `cmd/fabrica/add.go`), and the generator ties resource metadata to templates (see `pkg/codegen/generator.go`).

## How it works under the hood

When a resource is created or changed, handlers emit lifecycle events (see `pkg/events/events.go`). The reconciliation controller (`pkg/reconcile/controller.go`) subscribes and enqueues the affected resource. Your reconciler runs: it reads the resource from storage, checks Status, decides what to do, and writes back Status updates.

The handler template (`pkg/codegen/templates/server/handlers.go.tmpl`) includes a dedicated Status endpoint. That makes reconcilers safe and idempotent: they only write Status, while users write Spec. Storage backends (file or database) don’t matter here; reconcilers use the same storage client either way.

## Trade‑offs and limits

Reconciliation is eventual, not instant. Design reconcilers to be idempotent and safe to retry. Handle backoff and requeue. Expect out‑of‑order events. Don’t try to “fix” Spec from a controller—write to Status and let users update Spec.

The default event bus is in‑memory. It’s perfect for local development. For production, consider a durable bus. The controller uses a work queue; size and worker count set throughput and memory use.

## Try it

Here’s a tiny flow that turns on reconciliation and uses the generated client. This uses the file backend so it runs anywhere. If you add reconcilers (like in the rack example), you’ll see the controller act on new resources.

```bash
fabrica init rack-svc --module github.com/you/rack-svc --storage-type file --events --reconcile
fabrica add resource Rack
fabrica generate && go run ./cmd/server/ serve --data-dir ./data &
go run ./cmd/client/ rack create --spec '{"name":"rack-01"}'
```

If you prefer a database, switch to Ent at init and start the server with `--database-url "file:data/app.db?_fk=1"`. The `?_fk=1` pragma enables SQLite foreign keys.

## What to watch for in production

Make reconcilers idempotent. Use conditions on Status to record progress and errors. Keep steps small so you can retry safely. Choose a durable event bus if you need persistence or fan‑out. Tune worker counts and queue sizes to match your load. For the server, remember the `go run ./cmd/server/` trailing slash and ensure your data dir exists if you use the file backend.

## Related reading

Read how controllers are structured in `pkg/reconcile/controller.go` and how events are published in `pkg/events/events.go`. See how handlers separate Spec and Status in `pkg/codegen/templates/server/handlers.go.tmpl`. The CLI wiring lives in `cmd/fabrica/add.go`, and the generator in `pkg/codegen/generator.go`.

For a concrete walkthrough, check `docs/guides/reconciliation.md` and the example at `examples/04-rack-reconciliation/`.
