<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Conditional Requests, PATCH Operations, and Validation Example

This example demonstrates the integration of three powerful Fabrica features:

1. **Conditional Requests** (ETags, If-Match, If-None-Match)
2. **PATCH Operations** (JSON Merge Patch, JSON Patch, Shorthand Patch)
3. **Validation** (Struct tags + custom business logic)

## Features Demonstrated

### 1. Validation (Hybrid Approach)

The example shows **both** validation techniques:

#### Struct Tag Validation
```go
type Resource struct {
    Name   string `json:"name" validate:"required,k8sname,min=3,max=63"`
    Status string `json:"status" validate:"required,oneof=active inactive pending"`
    Tags   []string `json:"tags,omitempty" validate:"dive,labelvalue"`
}
```

#### Custom Validation Logic
```go
func (r *Resource) Validate(ctx context.Context) error {
    // Business rules that can't be expressed with tags
    if r.Status == "inactive" && len(r.Tags) > 0 {
        return errors.New("inactive resources cannot have tags")
    }

    if r.Status == "active" && !strings.HasPrefix(r.Name, "active-") {
        return fmt.Errorf("active resources must have names starting with 'active-'")
    }

    return nil
}
```

### 2. Conditional Requests

- **ETags**: Automatic generation and validation
- **If-Match**: Optimistic concurrency control
- **If-None-Match**: Efficient caching (304 Not Modified)
- **If-Modified-Since**: Time-based conditional requests
- **Cache-Control**: Proper caching directives

### 3. PATCH Operations

Three PATCH formats supported:

#### JSON Merge Patch (RFC 7386)
```bash
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"description":"Updated description"}'
```

#### JSON Patch (RFC 6902)
```bash
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/json-patch+json" \
  -d '[{"op":"replace","path":"/description","value":"New value"}]'
```

#### Shorthand Patch (Fabrica Extension)
```bash
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/shorthand-patch+json" \
  -d '{"metadata.version":"2.0","metadata.author":"admin"}'
```

## Running the Example

### Start the Server

```bash
go run main.go
```

The server starts on port 8080 with one example resource.

### Test Validation

#### Valid Resource Creation
```bash
curl -X POST http://localhost:8080/resources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "active-new-device",
    "status": "active",
    "description": "A valid resource"
  }' | jq
```

**Response** (201 Created):
```json
{
  "id": "1704230400000000000",
  "name": "active-new-device",
  "description": "A valid resource",
  "status": "active",
  "createdAt": "2025-01-03T10:00:00Z",
  "modifiedAt": "2025-01-03T10:00:00Z"
}
```

#### Invalid Name (Uppercase)
```bash
curl -X POST http://localhost:8080/resources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invalid-Name",
    "status": "active"
  }' | jq
```

**Response** (400 Bad Request):
```json
{
  "error": "Validation failed",
  "details": [
    {
      "field": "name",
      "tag": "k8sname",
      "value": "Invalid-Name",
      "message": "name must be a valid Kubernetes name (lowercase alphanumeric, -, or .)"
    }
  ]
}
```

#### Custom Validation Failure
```bash
curl -X POST http://localhost:8080/resources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "inactive-device",
    "status": "inactive",
    "tags": ["test"]
  }' | jq
```

**Response** (400 Bad Request):
```json
{
  "error": "inactive resources cannot have tags"
}
```

#### Missing Required Field
```bash
curl -X POST http://localhost:8080/resources \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Missing name"
  }' | jq
```

**Response** (400 Bad Request):
```json
{
  "error": "Validation failed",
  "details": [
    {
      "field": "name",
      "tag": "required",
      "message": "name is required"
    },
    {
      "field": "status",
      "tag": "required",
      "message": "status is required"
    }
  ]
}
```

### Test PATCH with Validation

#### Valid Merge Patch
```bash
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"description": "Updated via merge patch"}' | jq
```

**Response** (200 OK) - Resource updated successfully.

#### Invalid Status via Patch
```bash
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"status": "invalid-status"}' | jq
```

**Response** (400 Bad Request):
```json
{
  "error": "Validation failed",
  "details": [
    {
      "field": "status",
      "tag": "oneof",
      "value": "invalid-status",
      "message": "status must be one of: active inactive pending"
    }
  ]
}
```

#### Patch Violating Custom Rule
```bash
# First, change status to inactive
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"status": "inactive", "tags": []}' | jq

# Now try to add tags (should fail)
curl -X PATCH http://localhost:8080/resources/1 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"tags": ["test"]}' | jq
```

**Response** (400 Bad Request):
```json
{
  "error": "inactive resources cannot have tags"
}
```

### Test Conditional Requests

#### Get with ETag
```bash
curl -i http://localhost:8080/resources/1
```

**Response Headers**:
```
HTTP/1.1 200 OK
Content-Type: application/json
ETag: "abc123..."
Last-Modified: Wed, 03 Jan 2025 10:00:00 GMT
Cache-Control: public, max-age=300, must-revalidate
```

#### Conditional GET (Not Modified)
```bash
# Get the ETag from previous request
ETAG=$(curl -s -i http://localhost:8080/resources/1 | grep -i etag | cut -d' ' -f2 | tr -d '\r')

# Use it in If-None-Match
curl -i -H "If-None-Match: $ETAG" http://localhost:8080/resources/1
```

**Response**:
```
HTTP/1.1 304 Not Modified
ETag: "abc123..."
```

#### Optimistic Concurrency Control
```bash
# Get current ETag
ETAG=$(curl -s -i http://localhost:8080/resources/1 | grep -i etag | cut -d' ' -f2 | tr -d '\r')

# Update with If-Match (succeeds)
curl -X PATCH http://localhost:8080/resources/1 \
  -H "If-Match: $ETAG" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"description": "Updated with concurrency control"}' | jq

# Try to update with old ETag (fails with 412)
curl -i -X PATCH http://localhost:8080/resources/1 \
  -H "If-Match: $ETAG" \
  -H "Content-Type: application/merge-patch+json" \
  -d '{"description": "This will fail"}'
```

**Second Response**:
```
HTTP/1.1 412 Precondition Failed
```

## Validation Rules Demonstrated

### Struct Tag Validators

| Field | Rules | Examples |
|-------|-------|----------|
| `name` | `required,k8sname,min=3,max=63` | ✅ `active-device` ❌ `Device` ❌ `x` |
| `status` | `required,oneof=active inactive pending` | ✅ `active` ❌ `running` |
| `description` | `max=200` | ✅ Short text ❌ 201+ chars |
| `tags` | `dive,labelvalue` | ✅ `["v1","app"]` ❌ `["Tag_1"]` |

### Custom Business Rules

1. **Inactive resources cannot have tags**
   - Status = `inactive` → Tags must be empty

2. **Pending resources must have description**
   - Status = `pending` → Description required

3. **Active resources need prefix**
   - Status = `active` → Name must start with `active-`

## Integration Points

### Handler Pattern

```go
func createResource(w http.ResponseWriter, r *http.Request) {
    var resource Resource

    // 1. Decode JSON
    if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
        respondError(w, http.StatusBadRequest, err)
        return
    }

    // 2. Validate (hybrid: tags + custom)
    if err := validation.ValidateWithContext(r.Context(), &resource); err != nil {
        handleValidationError(w, err)  // Returns 400 with details
        return
    }

    // 3. Store resource
    resources[resource.ID] = &resource

    // 4. Set cache headers
    conditional.SetETag(w, etag)

    respondJSON(w, http.StatusCreated, resource)
}
```

### Error Response Format

```go
// Structured validation errors
{
  "error": "Validation failed",
  "details": [
    {
      "field": "name",
      "tag": "k8sname",
      "value": "Invalid_Name",
      "message": "name must be a valid Kubernetes name (lowercase alphanumeric, -, or .)"
    }
  ]
}

// Custom validation errors
{
  "error": "inactive resources cannot have tags"
}
```

## Key Takeaways

### 1. Validation Happens at Multiple Stages

- **CREATE**: Full validation before storing
- **UPDATE**: Full validation of new state
- **PATCH**: Validation after applying patch

### 2. Hybrid Validation is Powerful

- **Struct tags** for format/range validation
- **Custom logic** for business rules
- **Context-aware** for time-sensitive checks

### 3. Validation Integrates with HTTP Features

- Returns proper status codes (400 Bad Request)
- Provides detailed error information
- Works with all PATCH formats
- Compatible with conditional requests

### 4. Production Patterns

- Validate early (immediately after decode)
- Return structured errors for API clients
- Use context for timeouts/cancellation
- Combine with ETags for optimistic concurrency

## Further Reading

- [Validation Documentation](../../docs/validation.md)
- [Conditional Requests](../../docs/conditional-and-patch.md#conditional-requests)
- [PATCH Operations](../../docs/conditional-and-patch.md#patch-operations)
- [RFC 7232](https://tools.ietf.org/html/rfc7232) - HTTP Conditional Requests
- [RFC 7386](https://tools.ietf.org/html/rfc7386) - JSON Merge Patch
- [RFC 6902](https://tools.ietf.org/html/rfc6902) - JSON Patch
