---
title: OpenAPI Extensions
nav_order: 7
parent: Guides
---

# OpenAPI Extensions

Fabrica generates OpenAPI paths for resources it manages automatically. If your service also exposes custom routes (for example, compatibility endpoints or hand-written handlers), you can include those routes in the generated spec using a create-once extension hook.

## What Gets Generated

When you run:

```bash
fabrica generate
```

Fabrica writes:

- `cmd/server/openapi_generated.go` (overwritten every generation)
- `cmd/server/openapi_extensions.go` (created once, never overwritten)

The generated OpenAPI builder calls your hook at the end of spec generation:

```go
registerCustomOpenAPIPaths(spec)
```

## Why This Exists

Without an extension hook, custom routes are invisible in API docs and client tooling. The hook allows you to keep generated resource docs and manually maintained custom-route docs in one OpenAPI document.

## Add a Custom Path

Edit `cmd/server/openapi_extensions.go` and add path operations using `openapi3` types.

Example:

```go
package main

import "github.com/getkin/kin-openapi/openapi3"

func registerCustomOpenAPIPaths(spec *openapi3.T) {
	if spec.Paths == nil {
		spec.Paths = openapi3.NewPaths()
	}

	spec.Paths.Set("/healthz", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "getHealthz",
			Summary:     "Health check",
			Tags:        []string{"System"},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: openapi3.NewDescription("Service is healthy"),
				}}),
			),
		},
	})
}
```

## Regeneration Behavior

- Safe to re-run `fabrica generate`.
- `openapi_generated.go` is regenerated from templates.
- `openapi_extensions.go` is preserved exactly as you edited it.

This lets you keep custom OpenAPI path definitions under source control without merge churn from code generation.

## Best Practices

- Keep this file focused on path registration only.
- Group related routes under consistent tags.
- Use stable `OperationID` values to avoid client-generation churn.
- Document auth/security requirements for custom endpoints the same way as generated endpoints.
