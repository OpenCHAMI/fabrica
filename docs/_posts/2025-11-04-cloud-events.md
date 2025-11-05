---
layout: post
title: "Events as a Simple State Machine"
description: "How Fabrica’s event system lets you model and drive resource state changes."
author: "Alex Lovell-Troy"
---

Events let your API tell the world what changed. In Fabrica, that stream becomes more than a log. It is a simple way to model a resource’s state and move it forward. When a resource is created, updated, or deleted, the server publishes an event. When Status changes in a meaningful way, you can publish a condition change. Other parts of your system can subscribe and react.

## Under the hood

Generated handlers publish through helper functions in `pkg/events/events.go`. For local work you can use the in‑memory bus in `pkg/events/memory_bus.go`. The publish calls live next to handlers, not inside your business logic. Subscribers are normal functions that receive events and can call the generated client to update Status.

Think of a sensor that must be checked by a worker before it is ready. The user creates the sensor. The server saves it and publishes a created event. A subscriber hears that event, runs a check, and writes back a Status update that marks the sensor as ready or not. A ready condition produces its own event, and another subscriber may wake up and do the next step. You get a small, clear chain of actions without tight coupling.

This works well with the reconciliation pattern (`pkg/reconcile/controller.go`). A controller subscribes to events, enqueues work, and updates Status through the client’s status methods. Because Spec and Status live on different paths, workers do not conflict with users.

## Trade‑offs and limits

Events are best for edge‑triggered workflows and for integrating with outside systems. They are not a full workflow engine. Keep handlers small. Use reconciliation loops when you need to converge on a desired state.

## Try it

```bash
fabrica init sensors --events --events-bus memory
fabrica add resource Sensor
fabrica generate
go run ./cmd/client/ sensor create --spec '{"name":"s1","description":"first"}'
```

## What to watch for in production

Choose a bus that fits your stack. The in‑memory bus is for local testing. If you use SQLite for examples, enable foreign keys (`?_fk=1`). Keep subscribers small and testable; they should call the client’s status methods, not reach into storage.

## Related reading

- Guide: `docs/guides/events.md`
- Example: `examples/05-cloud-events/`
