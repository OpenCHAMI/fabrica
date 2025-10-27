<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Conditional Requests and PATCH Operations

This document describes the conditional request and PATCH operation support in Fabrica.

## Overview

Fabrica now includes comprehensive support for:

1. **Conditional Requests** (RFC 7232) - ETags, If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since
2. **JSON Merge Patch** (RFC 7386) - Simple merge-based partial updates
3. **JSON Patch** (RFC 6902) - Operation-based partial updates
4. **Shorthand Patches** - Simplified dot-notation patches

## Conditional Requests

Conditional requests allow clients to perform optimistic concurrency control and efficient caching.

### ETags

ETags are opaque identifiers that represent a specific version of a resource.

```go
import "github.com/openchami/fabrica/pkg/conditional"

// Generate ETag for resource
etag := conditional.DefaultETagGenerator(resourceData)

// Set ETag header
conditional.SetETag(w, etag)

// Check conditional headers
if conditional.CheckConditionalRequest(w, r, etag, lastModified) {
    return // Response already sent (304 or 412)
}
```

### Supported Headers

#### If-Match

Requires the resource's ETag to match. Used for safe updates.

```bash
# Update only if ETag matches
curl -X PUT http://localhost:8080/resources/123 \
  -H "If-Match: \"abc123\"" \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated"}'
```

**Response:**
- `200 OK` - ETag matched, update performed
- `412 Precondition Failed` - ETag didn't match, resource was modified

#### If-None-Match

Requires the resource's ETag to NOT match. Used for conditional GET.

```bash
# Get only if resource changed
curl -H "If-None-Match: \"abc123\"" \
  http://localhost:8080/resources/123
```

**Response:**
- `200 OK` - Resource changed, return new version
- `304 Not Modified` - Resource unchanged, save bandwidth

#### If-Unmodified-Since

Requires the resource to NOT be modified since the specified time.

```bash
curl -X PUT http://localhost:8080/resources/123 \
  -H "If-Unmodified-Since: Mon, 01 Jan 2024 00:00:00 GMT" \
  -d '{"name":"Updated"}'
```

**Response:**
- `200 OK` - Resource not modified since date
- `412 Precondition Failed` - Resource was modified

#### If-Modified-Since

Requires the resource to be modified since the specified time.

```bash
curl -H "If-Modified-Since: Mon, 01 Jan 2024 00:00:00 GMT" \
  http://localhost:8080/resources/123
```

**Response:**
- `200 OK` - Resource modified, return new version
- `304 Not Modified` - Resource not modified, save bandwidth

### ETag Middleware

Automatically add ETags to responses:

```go
import (
    "github.com/openchami/fabrica/pkg/conditional"
)

// Add ETag middleware to your router
router.Use(conditional.ETagMiddleware(nil)) // Uses default SHA-256 generator

// Or use custom ETag generator
customGen := func(data []byte) string {
    return fmt.Sprintf(`"v1-%x"`, md5.Sum(data))
}
router.Use(conditional.ETagMiddleware(customGen))
```

### Cache Control

Set caching directives:

```go
// No caching
conditional.SetCacheControl(w, conditional.CacheControlOptions{
    NoStore: true,
})

// Cache for 1 hour, must revalidate
conditional.SetCacheControl(w, conditional.CacheControlOptions{
    Public:         true,
    MaxAge:         3600,
    MustRevalidate: true,
})

// Private cache, immutable
conditional.SetCacheControl(w, conditional.CacheControlOptions{
    Private:   true,
    MaxAge:    86400,
    Immutable: true,
})
```

## PATCH Operations

PATCH operations enable partial updates to resources without sending the entire resource.

### JSON Merge Patch (RFC 7386)

The simplest form - just merge the patch into the original.

**Request:**
```bash
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "name": "Updated Name",
    "description": "New description"
  }'
```

**Behavior:**
- Fields in patch overwrite corresponding fields in original
- `null` values delete fields
- Missing fields remain unchanged

**Example:**
```json
// Original
{"name":"John","age":30,"city":"NYC"}

// Patch
{"age":31,"city":null}

// Result
{"name":"John","age":31}
```

**Code:**
```go
import "github.com/openchami/fabrica/pkg/patch"

updated, err := patch.ApplyMergePatch(original, patchData)
if err != nil {
    // Handle error
}
```

### JSON Patch (RFC 6902)

Operation-based patches with precise control.

**Request:**
```bash
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/json-patch+json" \
  -d '[
    {"op":"replace","path":"/name","value":"New Name"},
    {"op":"add","path":"/email","value":"user@example.com"},
    {"op":"remove","path":"/age"}
  ]'
```

**Operations:**

| Operation | Description | Example |
|-----------|-------------|---------|
| `add` | Add a value | `{"op":"add","path":"/email","value":"user@example.com"}` |
| `remove` | Remove a value | `{"op":"remove","path":"/age"}` |
| `replace` | Replace a value | `{"op":"replace","path":"/name","value":"Jane"}` |
| `move` | Move a value | `{"op":"move","from":"/name","path":"/fullName"}` |
| `copy` | Copy a value | `{"op":"copy","from":"/name","path":"/displayName"}` |
| `test` | Test a value | `{"op":"test","path":"/age","value":30}` |

**Code:**
```go
import "github.com/openchami/fabrica/pkg/patch"

// Apply JSON Patch
updated, err := patch.ApplyJSONPatch(original, patchData)

// Validate patch before applying
if err := patch.ValidateJSONPatch(patchData); err != nil {
    // Invalid patch
}
```

### Shorthand Patches

Simplified dot-notation patches for convenience.

**Request:**
```bash
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/shorthand-patch+json" \
  -d '{
    "user.name": "Jane",
    "user.age": 31,
    "user.city": null
  }'
```

**Behavior:**
- Dot notation represents nested paths
- `null` values remove fields
- Automatically converted to JSON Patch operations

**Example:**
```json
// Original
{"user":{"name":"John","age":30,"city":"NYC"}}

// Shorthand Patch
{"user.age":31,"user.city":null}

// Result
{"user":{"name":"John","age":31}}
```

**Code:**
```go
import "github.com/openchami/fabrica/pkg/patch"

updated, err := patch.ApplyShorthandPatch(original, patchData)
```

## Handler Integration

### Manual Integration

```go
func UpdateResource(w http.ResponseWriter, r *http.Request) {
    uid := chi.URLParam(r, "uid")

    // Load current resource
    original, err := storage.LoadResource(uid)
    if err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }

    // Marshal to JSON
    originalJSON, _ := json.Marshal(original)

    // Check conditional headers
    etag := conditional.DefaultETagGenerator(originalJSON)
    lastModified := original.Metadata.ModifiedAt

    if conditional.CheckConditionalRequest(w, r, etag, lastModified) {
        return // Response already sent
    }

    // Handle PATCH
    if r.Method == http.MethodPatch {
        patchData, _ := io.ReadAll(r.Body)
        patchType := patch.DetectPatchType(r.Header.Get("Content-Type"))

        updated, err := patch.ApplyPatch(originalJSON, patchData, patchType)
        if err != nil {
            respondError(w, http.StatusUnprocessableEntity, err)
            return
        }

        // Unmarshal back to resource
        if err := json.Unmarshal(updated, &original); err != nil {
            respondError(w, http.StatusInternalServerError, err)
            return
        }
    } else {
        // Handle PUT normally
        json.NewDecoder(r.Body).Decode(&original)
    }

    // Save and return
    storage.SaveResource(original)

    newETag := conditional.DefaultETagGenerator(originalJSON)
    conditional.SetETag(w, newETag)
    conditional.SetLastModified(w, original.Metadata.ModifiedAt)

    respondJSON(w, http.StatusOK, original)
}
```

### Using Middleware

```go
import (
    "github.com/openchami/fabrica/pkg/patch"
)

// Automatic PATCH middleware
handler := patch.PatchMiddleware(
    func(r *http.Request) ([]byte, error) {
        // Get resource
        uid := chi.URLParam(r, "uid")
        resource, err := storage.LoadResource(uid)
        if err != nil {
            return nil, err
        }
        return json.Marshal(resource)
    },
    func(r *http.Request, data []byte) error {
        // Save resource
        var resource Resource
        json.Unmarshal(data, &resource)
        return storage.SaveResource(resource)
    },
)(yourHandler)
```

### Generated Handler Support

Update your handler template to include PATCH support:

```gotmpl
// Update{{.Name}} updates an existing {{.Name}} resource
func Update{{.Name}}(w http.ResponseWriter, r *http.Request) {
    uid := chi.URLParam(r, "uid")

    {{camelCase .Name}}, err := storage.Load{{.StorageName}}(uid)
    if err != nil {
        respondError(w, http.StatusNotFound, err)
        return
    }

    // Marshal current state
    currentJSON, _ := json.Marshal({{camelCase .Name}})

    // Generate ETag and check conditionals
    etag := conditional.DefaultETagGenerator(currentJSON)
    if conditional.CheckConditionalRequest(w, r, etag, {{camelCase .Name}}.Metadata.ModifiedAt) {
        return
    }

    // Handle PATCH
    if r.Method == http.MethodPatch {
        patchData, _ := io.ReadAll(r.Body)
        patchType := patch.DetectPatchType(r.Header.Get("Content-Type"))

        updatedJSON, err := patch.ApplyPatch(currentJSON, patchData, patchType)
        if err != nil {
            respondError(w, http.StatusUnprocessableEntity, err)
            return
        }

        json.Unmarshal(updatedJSON, &{{camelCase .Name}})
    } else {
        // Normal PUT
        var req Update{{.Name}}Request
        json.NewDecoder(r.Body).Decode(&req)
        req.ApplyTo{{.Name}}({{camelCase .Name}})
    }

    {{camelCase .Name}}.Touch()
    storage.Save{{.StorageName}}({{camelCase .Name}})

    // Set response headers
    updatedJSON, _ := json.Marshal({{camelCase .Name}})
    newETag := conditional.DefaultETagGenerator(updatedJSON)
    conditional.SetETag(w, newETag)
    conditional.SetLastModified(w, {{camelCase .Name}}.Metadata.ModifiedAt)

    respondJSON(w, http.StatusOK, {{camelCase .Name}})
}
```

## Advanced Features

### Dry Run

Test patches without applying them:

```go
opts := patch.PatchOptions{
    DryRun: true,
}

result, err := patch.ApplyPatchWithOptions(original, patchData, patchType, opts)
// result.Changes contains list of changed paths
// result.Updated equals result.Original (not modified)
```

### Field Masks

Restrict which fields can be patched:

```go
opts := patch.PatchOptions{
    FieldMask: []string{"spec", "metadata.labels"},
}

result, err := patch.ApplyPatchWithOptions(original, patchData, patchType, opts)
// Only spec and metadata.labels can be patched
// Returns error if patch touches other fields
```

### Optimistic Concurrency

Combine ETags with PATCH for safe concurrent updates:

```bash
# 1. Get current version with ETag
curl -i http://localhost:8080/resources/123
# ETag: "abc123"

# 2. Patch with If-Match to ensure no concurrent modifications
curl -X PATCH http://localhost:8080/resources/123 \
  -H "If-Match: \"abc123\"" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"status":"active"}'
```

### Compute Changes

Get a list of what changed:

```go
changes, err := patch.ComputePatchChanges(original, updated)
// changes = ["/name", "/age", "/metadata/modifiedAt"]
```

### Create Patches

Generate patches from two versions:

```go
patchData, err := patch.CreatePatch(oldVersion, newVersion)
// Returns JSON Merge Patch that transforms oldVersion to newVersion
```

## Client Usage

### Using curl

```bash
# Conditional GET with ETag
curl -i http://localhost:8080/resources/123
# Note the ETag in response

curl -H "If-None-Match: \"abc123\"" \
  http://localhost:8080/resources/123
# Returns 304 if unchanged

# JSON Merge Patch
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/merge-patch+json" \
  -H "If-Match: \"abc123\"" \
  -d '{"name":"Updated"}'

# JSON Patch
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/json-patch+json" \
  -d '[{"op":"replace","path":"/status","value":"active"}]'

# Shorthand Patch
curl -X PATCH http://localhost:8080/resources/123 \
  -H "Content-Type: application/shorthand-patch+json" \
  -d '{"spec.replicas":3,"status.phase":"Running"}'
```

### Using Go Client

```go
import (
    "github.com/openchami/fabrica/pkg/client"
    "github.com/openchami/fabrica/pkg/patch"
)

// Get with ETag support
resp, etag, err := client.GetWithETag(ctx, "/resources/123")

// Patch with optimistic concurrency
patchData, _ := patch.MergePatchFromMap(map[string]interface{}{
    "status": "active",
})

err = client.PatchWithETag(ctx, "/resources/123", patchData, etag, patch.JSONMergePatch)
if err == client.ErrPreconditionFailed {
    // Resource was modified, retry
}
```

## Best Practices

1. **Always use ETags for updates** - Prevents lost updates in concurrent scenarios
2. **Use JSON Merge Patch for simple updates** - Simpler and more intuitive
3. **Use JSON Patch for complex operations** - More control, atomic operations
4. **Set proper Cache-Control headers** - Optimize bandwidth and performance
5. **Validate patches before applying** - Catch errors early
6. **Use field masks for sensitive fields** - Prevent unauthorized modifications
7. **Include Accept-Patch header** - Advertise supported patch types
8. **Handle 412 Precondition Failed** - Implement retry logic
9. **Use dry-run for testing** - Validate patches without side effects
10. **Log patch operations** - Audit trail for changes

## Error Handling

| Status Code | Description | When It Occurs |
|-------------|-------------|----------------|
| `304 Not Modified` | Resource unchanged | If-None-Match matches on GET |
| `400 Bad Request` | Invalid patch | Malformed JSON or patch operations |
| `412 Precondition Failed` | Condition not met | ETag mismatch, resource modified |
| `422 Unprocessable Entity` | Patch can't be applied | Invalid operation, path not found |
| `428 Precondition Required` | Missing required header | If-Match required but not provided |

## See Also

- [RFC 7232 - HTTP Conditional Requests](https://tools.ietf.org/html/rfc7232)
- [RFC 7386 - JSON Merge Patch](https://tools.ietf.org/html/rfc7386)
- [RFC 6902 - JSON Patch](https://tools.ietf.org/html/rfc6902)
- [Optimistic Concurrency Control](https://en.wikipedia.org/wiki/Optimistic_concurrency_control)
