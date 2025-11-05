---
layout: post
title: "When Standards Live in Code"
description: "Why encoding REST rules in generators leads to steadier, more supportable APIs."
author: "Alex Lovell-Troy"
---

Many API teams say they follow REST, but each team writes it a little differently. Over time, small differences pile up. One service wraps list results in an object, another returns an array. One updates status together with spec, another splits them. These choices matter to users, and they matter even more when you have to support the API for years.

Fabrica takes a different path. Instead of asking every team to remember the same rules, it puts the rules in code. The generator writes handlers, routes, and models that follow the same standards every time. You can see those standards in `pkg/codegen/templates/server/routes.go.tmpl` and `pkg/codegen/templates/server/handlers.go.tmpl`. Lists are arrays by design. Status gets its own endpoints. The OpenAPI file (`pkg/codegen/templates/server/openapi.go.tmpl`) comes from the same source, so docs match behavior.

The split between Spec and Status is not a note in a doc; it is code. Status‑only handlers write through a separate path, and status updates can publish events. Resource code embeds common metadata from `pkg/resource`. Validation happens through `pkg/validation`. PATCH and conditional logic live in `pkg/patch` and `pkg/conditional`. This means your API follows the same patterns even as teams rotate.

Before vs after maintenance tells the story. Before, a new teammate had to learn each service’s house rules. After, they learn the framework once. They know where routes are defined. They know list responses are arrays. They know not to mix spec and status. Reviews focus on domain logic, not handler shape. That lowers support costs.

When you adopt a new behavior, you update the generator, not dozens of repos. Regeneration applies the change evenly. Examples and integration tests in this repository cover the common paths and catch regressions. The source of truth is a template, and it updates all services the same way.

Try it

```bash
fabrica init myapp
fabrica add resource Device
fabrica generate
go run ./cmd/client/ device update-status <uid> --spec '{"ready":true}' -o json
```

What to watch for in production

Keep spec and status separate in your own code. Use the generated status methods in the client for controllers and workers. If you use a database backend, follow the storage guide for setup (for SQLite, enable foreign keys with `?_fk=1`).

Related reading

- Guide: `docs/guides/resource-model.md`
- Example: `examples/06-status-subresource/`
