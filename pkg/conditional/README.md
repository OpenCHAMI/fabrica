<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Conditional Requests and PATCH Operations

This package provides comprehensive support for HTTP conditional requests and PATCH operations in Fabrica.

## Features

### Conditional Requests (RFC 7232)

- ✅ **ETag generation** - SHA-256 based, strong and weak ETags
- ✅ **If-Match** - Optimistic concurrency control
- ✅ **If-None-Match** - Efficient caching
- ✅ **If-Modified-Since** - Bandwidth optimization
- ✅ **If-Unmodified-Since** - Safe updates
- ✅ **Cache-Control** - Fine-grained caching directives
- ✅ **Last-Modified** - Resource modification timestamps
- ✅ **Vary** - Response variation headers

### PATCH Operations

- ✅ **JSON Merge Patch** (RFC 7386) - Simple merge-based updates
- ✅ **JSON Patch** (RFC 6902) - Operation-based updates with add, remove, replace, move, copy, test
- ✅ **Shorthand Patches** - Dot-notation convenience format
- ✅ **Automatic PATCH from GET+PUT** - Generate PATCH handlers automatically
- ✅ **Dry-run mode** - Test patches without applying
- ✅ **Field masks** - Restrict which fields can be patched
- ✅ **Change tracking** - Compute list of changed paths
- ✅ **Validation** - Pre-validate patches before applying

## Packages

### `pkg/conditional`

HTTP conditional request support with ETags and cache control.

```go
import "github.com/alexlovelltroy/fabrica/pkg/conditional"

// Generate ETag
etag := conditional.DefaultETagGenerator(resourceData)
conditional.SetETag(w, etag)

// Check conditional headers
if conditional.CheckConditionalRequest(w, r, etag, lastModified) {
    return // Response sent (304 or 412)
}

// Set cache control
conditional.SetCacheControl(w, conditional.CacheControlOptions{
    Public:         true,
    MaxAge:         3600,
    MustRevalidate: true,
})
```

### `pkg/patch`

PATCH operation support with multiple formats.

```go
import "github.com/alexlovelltroy/fabrica/pkg/patch"

// JSON Merge Patch
updated, err := patch.ApplyMergePatch(original, patchData)

// JSON Patch
updated, err := patch.ApplyJSONPatch(original, patchData)

// Shorthand Patch
updated, err := patch.ApplyShorthandPatch(original, patchData)

// Auto-detect and apply
patchType := patch.DetectPatchType(r.Header.Get("Content-Type"))
updated, err := patch.ApplyPatch(original, patchData, patchType)
```

## Quick Start

### 1. Basic Usage

```go
package main

import (
    "encoding/json"
    "net/http"
    
    "github.com/alexlovelltroy/fabrica/pkg/conditional"
    "github.com/alexlovelltroy/fabrica/pkg/patch"
    "github.com/go-chi/chi/v5"
)

func UpdateResource(w http.ResponseWriter, r *http.Request) {
    // Load current resource
    resource, _ := loadResource(chi.URLParam(r, "id"))
    originalJSON, _ := json.Marshal(resource)
    
    // Check conditional headers
    etag := conditional.DefaultETagGenerator(originalJSON)
    if conditional.CheckConditionalRequest(w, r, etag, resource.ModifiedAt) {
        return
    }
    
    // Handle PATCH
    if r.Method == http.MethodPatch {
        patchData, _ := io.ReadAll(r.Body)
        patchType := patch.DetectPatchType(r.Header.Get("Content-Type"))
        
        updated, err := patch.ApplyPatch(originalJSON, patchData, patchType)
        if err != nil {
            http.Error(w, err.Error(), http.StatusUnprocessableEntity)
            return
        }
        
        json.Unmarshal(updated, &resource)
    }
    
    // Save and return
    saveResource(resource)
    newETag := conditional.DefaultETagGenerator(updated)
    conditional.SetETag(w, newETag)
    json.NewEncoder(w).Encode(resource)
}
```

### 2. Using Middleware

```go
// Add ETag middleware
router.Use(conditional.ETagMiddleware(nil))

// Auto-PATCH middleware
router.Use(patch.AutoPatchMiddleware("/api/resources"))
```

### 3. Example Requests

```bash
# Get resource with ETag
curl -i http://localhost:8080/resources/123
# Note the ETag: "abc123"

# Conditional GET (304 if unchanged)
curl -H "If-None-Match: \"abc123\"" \
  http://localhost:8080/resources/123

# Update with optimistic concurrency
curl -X PATCH http://localhost:8080/resources/123 \
  -H "If-Match: \"abc123\"" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"status":"active"}'

# JSON Patch operations
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/json-patch+json" \
  -d '[
    {"op":"replace","path":"/status","value":"inactive"},
    {"op":"add","path":"/tags/-","value":"archived"}
  ]'

# Shorthand patch
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/shorthand-patch+json" \
  -d '{"spec.replicas":3,"status.phase":"Running"}'
```

## Examples

See `examples/conditional-patch/main.go` for a complete working example.

To run:

```bash
cd examples/conditional-patch
go run main.go
```

Then test with:

```bash
# Get resource
curl -i http://localhost:8080/resources/1

# JSON Merge Patch
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"status":"inactive","description":"Updated"}'

# JSON Patch
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/json-patch+json" \
  -d '[{"op":"replace","path":"/status","value":"pending"}]'
```

## Documentation

- [Conditional and PATCH Guide](../docs/conditional-and-patch.md) - Comprehensive usage guide
- [API Reference](https://pkg.go.dev/github.com/alexlovelltroy/fabrica/pkg/conditional) - Package documentation
- [Examples](../examples/conditional-patch/) - Working examples

## Supported RFCs

- [RFC 7232](https://tools.ietf.org/html/rfc7232) - HTTP/1.1 Conditional Requests
- [RFC 7386](https://tools.ietf.org/html/rfc7386) - JSON Merge Patch
- [RFC 6902](https://tools.ietf.org/html/rfc6902) - JavaScript Object Notation (JSON) Patch
- [RFC 7234](https://tools.ietf.org/html/rfc7234) - HTTP/1.1 Caching

## Comparison with Other Frameworks

| Feature | Fabrica | Huma | Go-Fuego | Standard Chi/Gin |
|---------|---------|------|----------|------------------|
| ETag Support | ✅ Built-in | ✅ Built-in | ❌ Manual | ❌ Manual |
| JSON Merge Patch | ✅ Built-in | ✅ Built-in | ❌ Manual | ❌ Manual |
| JSON Patch | ✅ Built-in | ✅ Built-in | ❌ Manual | ❌ Manual |
| Shorthand Patch | ✅ Built-in | ❌ No | ❌ No | ❌ No |
| Auto PATCH | ✅ Yes | ✅ Yes | ❌ No | ❌ No |
| Conditional Requests | ✅ Full | ✅ Full | ❌ Manual | ❌ Manual |
| Cache Control | ✅ Built-in | ✅ Built-in | ❌ Manual | ❌ Manual |
| Field Masks | ✅ Built-in | ❌ No | ❌ No | ❌ No |
| Dry Run | ✅ Built-in | ❌ No | ❌ No | ❌ No |

## Testing

Run tests:

```bash
go test ./pkg/conditional/...
go test ./pkg/patch/...
```

With coverage:

```bash
go test -cover ./pkg/conditional/...
go test -cover ./pkg/patch/...
```

## Best Practices

1. **Always use ETags for updates** - Prevents lost updates
2. **Set appropriate Cache-Control** - Optimize bandwidth
3. **Validate patches before applying** - Catch errors early
4. **Use field masks for sensitive data** - Security
5. **Handle 412 gracefully** - Implement retry logic
6. **Log patch operations** - Audit trail
7. **Use JSON Merge Patch for simple updates** - Easier for clients
8. **Use JSON Patch for complex operations** - More control
9. **Test with dry-run first** - Validate changes
10. **Document supported patch types** - Use Accept-Patch header

## Contributing

Contributions welcome! Please ensure:

- All tests pass
- Code follows Go conventions
- Documentation is updated
- Examples are provided for new features

## License

Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
