<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Client API Guide

This guide explains how to use the generated Fabrica Go client library, covering both the simple and advanced APIs.

## Overview

Fabrica generates two client API styles for creating and updating resources:

1. **Simple API** - Minimal boilerplate for common operations (recommended for most use cases)
2. **Advanced API** - Full control over resource envelopes for complex scenarios

Both APIs coexist without breaking changes, allowing you to choose the right tool for each situation.

## Quick Start

### Installation

After running `fabrica generate --client`, the client package is available at:

```go
import "github.com/your-org/your-project/pkg/client"
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/your-org/device-inventory/pkg/client"
    v1 "github.com/your-org/device-inventory/apis/example.fabrica.dev/v1"
)

func main() {
    ctx := context.Background()

    // Create client
    c, err := client.NewClient("http://localhost:8080", nil, client.DefaultLogger())
    if err != nil {
        log.Fatal(err)
    }

    // Simple API - just name + spec
    spec := v1.DeviceSpec{
        Description: "Core switch",
        IPAddr:      "192.168.1.10",
    }

    device, err := c.CreateDeviceSimple(ctx, "switch-01", spec)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created device: %s (UID: %s)", device.Metadata.Name, device.Metadata.UID)
}
```

## Simple API

### Overview

The simple API provides streamlined methods that only require a name and spec. The client automatically handles the resource envelope (APIVersion, Kind, Metadata).

### Create Operation

**Method signature:**
```go
func (c *Client) CreateDeviceSimple(ctx context.Context, name string, spec DeviceSpec) (*Device, error)
```

**Example:**
```go
spec := v1.DeviceSpec{
    Description: "Core network switch",
    IPAddr:      "192.168.1.10",
    Location:    "DataCenter A",
}

device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)
if err != nil {
    log.Fatal(err)
}
```

**What happens internally:**
- Client constructs a `CreateDeviceRequest` with minimal metadata
- Sets `Metadata.Name` from the name parameter
- Copies the spec as-is
- Server generates UID, timestamps, APIVersion, and Kind

### Update Operation

**Method signature:**
```go
func (c *Client) UpdateDeviceSimple(ctx context.Context, uid string, spec DeviceSpec) (*Device, error)
```

**Example:**
```go
updatedSpec := v1.DeviceSpec{
    Description: "Updated description",
    IPAddr:      "192.168.1.20",
}

device, err := client.UpdateDeviceSimple(ctx, device.Metadata.UID, updatedSpec)
if err != nil {
    log.Fatal(err)
}
```

**What happens internally:**
- Client constructs an `UpdateDeviceRequest` with only the spec
- Server preserves existing metadata (name, labels, annotations, UID, timestamps)
- Only the spec fields are updated

### When to Use Simple API

✅ **Use the simple API when:**
- You only need to set the resource name and spec
- You don't need custom labels or annotations
- You want minimal boilerplate code
- You're writing straightforward CRUD operations
- You're new to the API and want the easiest path

❌ **Don't use the simple API when:**
- You need to set labels for organizing resources
- You need annotations for metadata
- You're migrating code that already constructs full requests
- You need to update metadata fields along with the spec

## Advanced API

### Overview

The advanced API provides full control over the resource envelope, including labels and annotations. This is useful for complex scenarios requiring metadata management.

### Create Operation

**Method signature:**
```go
func (c *Client) CreateDevice(ctx context.Context, req CreateDeviceRequest) (*Device, error)
```

**Request structure:**
```go
type CreateDeviceRequest struct {
    Metadata    fabrica.Metadata  `json:"metadata" validate:"required"`
    Spec        DeviceSpec        `json:"spec" validate:"required"`
    Labels      map[string]string `json:"labels,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty"`
}
```

**Example:**
```go
req := client.CreateDeviceRequest{
    Metadata: fabrica.Metadata{
        Name: "switch-01",
    },
    Spec: v1.DeviceSpec{
        Description: "Core switch",
        IPAddr:      "192.168.1.10",
    },
    Labels: map[string]string{
        "environment": "production",
        "datacenter":  "us-west-2",
        "team":        "networking",
    },
    Annotations: map[string]string{
        "deployment.notes":    "Deployed during maintenance window",
        "contact.oncall":      "network-team@example.com",
        "cost.center":         "IT-1234",
    },
}

device, err := client.CreateDevice(ctx, req)
if err != nil {
    log.Fatal(err)
}
```

### Update Operation

**Method signature:**
```go
func (c *Client) UpdateDevice(ctx context.Context, uid string, req UpdateDeviceRequest) (*Device, error)
```

**Request structure:**
```go
type UpdateDeviceRequest struct {
    Metadata    fabrica.Metadata  `json:"metadata,omitempty"`
    Spec        DeviceSpec        `json:"spec,omitempty"`
    Labels      map[string]string `json:"labels,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty"`
}
```

**Example:**
```go
req := client.UpdateDeviceRequest{
    Spec: v1.DeviceSpec{
        Description: "Updated description",
        IPAddr:      "192.168.1.20",
    },
    Labels: map[string]string{
        "environment": "staging", // Change environment label
    },
}

device, err := client.UpdateDevice(ctx, device.Metadata.UID, req)
if err != nil {
    log.Fatal(err)
}
```

### When to Use Advanced API

✅ **Use the advanced API when:**
- You need to set labels for resource organization
- You need annotations for storing metadata
- You're updating labels or annotations along with the spec
- You need full control over what metadata gets sent
- You're integrating with systems that rely on labels/annotations

## Feature Comparison

| Feature | Simple API | Advanced API |
|---------|-----------|--------------|
| **Method name** | `CreateDeviceSimple()` / `UpdateDeviceSimple()` | `CreateDevice()` / `UpdateDevice()` |
| **Name** | ✅ Parameter | ✅ In `request.Metadata.Name` |
| **Spec** | ✅ Parameter | ✅ In `request.Spec` |
| **Labels** | ❌ Not supported | ✅ In `request.Labels` |
| **Annotations** | ❌ Not supported | ✅ In `request.Annotations` |
| **UID** | 🔒 Server-generated | 🔒 Server-generated (user value ignored) |
| **Timestamps** | 🔒 Server-generated | 🔒 Server-generated (user value ignored) |
| **APIVersion** | 🔄 Client auto-set | 🔄 Client auto-set |
| **Kind** | 🔄 Client auto-set | 🔄 Client auto-set |
| **Boilerplate** | Minimal | More verbose |
| **Use case** | 90% of operations | Advanced metadata control |

## Common Patterns

### Pattern 1: Basic CRUD with Simple API

```go
// Create
spec := v1.DeviceSpec{
    Description: "Core switch",
    IPAddr:      "192.168.1.10",
}
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)

// Read
device, err = client.GetDevice(ctx, device.Metadata.UID)

// Update
spec.Description = "Updated description"
device, err = client.UpdateDeviceSimple(ctx, device.Metadata.UID, spec)

// Delete
err = client.DeleteDevice(ctx, device.Metadata.UID)
```

### Pattern 2: Resource Organization with Labels

```go
// Create devices in production environment
for i := 1; i <= 5; i++ {
    req := client.CreateDeviceRequest{
        Metadata: fabrica.Metadata{
            Name: fmt.Sprintf("prod-switch-%02d", i),
        },
        Spec: v1.DeviceSpec{
            Description: fmt.Sprintf("Production switch %d", i),
            IPAddr:      fmt.Sprintf("10.0.1.%d", i+10),
        },
        Labels: map[string]string{
            "environment": "production",
            "rack":        fmt.Sprintf("R%d", (i-1)/2+1),
        },
    }

    _, err := client.CreateDevice(ctx, req)
    if err != nil {
        log.Printf("Failed to create device %d: %v", i, err)
    }
}

// Later: Query devices by label (requires server-side filtering)
// This demonstrates why labels are valuable for organization
devices, err := client.GetDevices(ctx)
for _, device := range devices {
    if device.Metadata.Labels["environment"] == "production" {
        fmt.Printf("Production device: %s\n", device.Metadata.Name)
    }
}
```

### Pattern 3: Migration Metadata with Annotations

```go
// Track migration metadata with annotations
req := client.CreateDeviceRequest{
    Metadata: fabrica.Metadata{
        Name: "legacy-switch-01",
    },
    Spec: v1.DeviceSpec{
        Description: "Migrated from legacy system",
        IPAddr:      "192.168.1.100",
    },
    Annotations: map[string]string{
        "migration.source-system":    "legacy-inventory",
        "migration.source-id":        "SW-12345",
        "migration.date":             "2026-01-15",
        "migration.performed-by":     "migration-script-v2",
        "legacy.last-maintenance":    "2025-12-01",
    },
}

device, err := client.CreateDevice(ctx, req)
```

### Pattern 4: Mixing Simple and Advanced APIs

```go
// Use simple API for routine operations
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)

// Switch to advanced API when you need labels
if isProduction {
    updateReq := client.UpdateDeviceRequest{
        Spec: spec,
        Labels: map[string]string{
            "environment": "production",
            "criticality": "high",
        },
    }
    device, err = client.UpdateDevice(ctx, device.Metadata.UID, updateReq)
}
```

## Client Configuration

### Creating a Client

**Basic client:**
```go
client, err := client.NewClient("http://localhost:8080", nil, client.DefaultLogger())
```

**With custom HTTP client:**
```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}

client, err := client.NewClient("http://localhost:8080", httpClient, client.DefaultLogger())
```

**With JWT authentication:**
```go
token := "eyJhbGciOiJIUzI1NiIs..."
client, err := client.NewClientWithBearerToken(
    "http://localhost:8080",
    token,
    nil,
    client.DefaultLogger(),
)
```

**With API versioning:**
```go
client, err := client.NewClient("http://localhost:8080", nil, client.DefaultLogger())
versionedClient := client.WithVersion("v1")
```

### Logging Configuration

**Default logger (warnings only):**
```go
logger := client.DefaultLogger()
```

**Custom log level:**
```go
logger, err := client.NewLogger(client.LogLevelDebug)
client, err := client.NewClient("http://localhost:8080", nil, logger)
```

**Available log levels:**
- `LogLevelDebug` - Detailed request/response logging
- `LogLevelInfo` - Informational messages
- `LogLevelWarning` - Warnings only (default)

## Error Handling

### Client Errors

```go
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)
if err != nil {
    // Check for specific error types
    switch {
    case strings.Contains(err.Error(), "validation failed"):
        log.Printf("Validation error: %v", err)
        // Handle validation errors
    case strings.Contains(err.Error(), "API error (409)"):
        log.Printf("Conflict: device already exists")
        // Handle conflicts
    case strings.Contains(err.Error(), "API error (404)"):
        log.Printf("Not found: device doesn't exist")
        // Handle not found
    default:
        log.Printf("Unexpected error: %v", err)
    }
}
```

### Validation Errors

```go
// Validation errors come from the server
device, err := client.CreateDeviceSimple(ctx, "switch-01", v1.DeviceSpec{
    IPAddr: "invalid-ip", // Will fail validation
})
if err != nil {
    log.Printf("Server validation failed: %v", err)
    // Error message contains validation details from server
}
```

## Migration Guide

### Migrating from Advanced to Simple API

**Before:**
```go
req := client.CreateDeviceRequest{
    Metadata: fabrica.Metadata{
        Name: "device-001",
    },
    Spec: spec,
}
device, err := client.CreateDevice(ctx, req)
```

**After:**
```go
device, err := client.CreateDeviceSimple(ctx, "device-001", spec)
```

**Migration checklist:**
1. ✅ Identify operations that don't use labels/annotations
2. ✅ Replace `CreateDevice(ctx, req)` with `CreateDeviceSimple(ctx, name, spec)`
3. ✅ Replace `UpdateDevice(ctx, uid, req)` with `UpdateDeviceSimple(ctx, uid, spec)`
4. ✅ Test to ensure behavior is identical
5. ✅ Keep advanced API for operations that need labels/annotations

### Coexistence Strategy

Both APIs work together seamlessly:

```go
// Create with simple API
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)

// Add labels later with advanced API
updateReq := client.UpdateDeviceRequest{
    Labels: map[string]string{
        "environment": "production",
    },
}
device, err = client.UpdateDevice(ctx, device.Metadata.UID, updateReq)

// Continue using simple API for spec-only updates
device, err = client.UpdateDeviceSimple(ctx, device.Metadata.UID, newSpec)
```

## Best Practices

### 1. Start with Simple API

Default to the simple API for new code:

```go
// ✅ Good: Simple and clear
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)

// ❌ Unnecessary: Using advanced API without labels/annotations
req := client.CreateDeviceRequest{
    Metadata: fabrica.Metadata{Name: "switch-01"},
    Spec: spec,
}
device, err := client.CreateDevice(ctx, req)
```

### 2. Use Labels for Organization

When you do need labels, be consistent:

```go
// ✅ Good: Consistent label keys
Labels: map[string]string{
    "environment": "production",
    "team":        "networking",
    "datacenter":  "us-west-2",
}

// ❌ Bad: Inconsistent naming
Labels: map[string]string{
    "env":          "production",
    "Team":         "networking",
    "data_center":  "us-west-2",
}
```

### 3. Use Annotations for Metadata

Store non-queryable metadata in annotations:

```go
// ✅ Good: Rich metadata in annotations
Annotations: map[string]string{
    "deployment.notes":     "Deployed during scheduled maintenance",
    "contact.oncall":       "network-team@example.com",
    "cost.center":          "IT-1234",
    "monitoring.dashboard": "https://grafana.example.com/d/abc123",
}
```

### 4. Don't Mix Concerns

Keep spec and metadata separate:

```go
// ✅ Good: Spec contains domain data only
type DeviceSpec struct {
    Description string `json:"description"`
    IPAddr      string `json:"ipaddr"`
    Location    string `json:"location"`
}

// ❌ Bad: Mixing metadata into spec
type DeviceSpec struct {
    Description string            `json:"description"`
    IPAddr      string            `json:"ipaddr"`
    Labels      map[string]string `json:"labels"` // Belongs in metadata!
}
```

## Troubleshooting

### Issue: "validation failed: name is required"

**Cause:** Empty name parameter in simple API

**Fix:**
```go
// ❌ Wrong
device, err := client.CreateDeviceSimple(ctx, "", spec)

// ✅ Correct
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)
```

### Issue: Labels not being set

**Cause:** Using simple API for operation that needs labels

**Fix:**
```go
// ❌ Wrong: Simple API doesn't support labels
device, err := client.CreateDeviceSimple(ctx, "switch-01", spec)
// Labels are not set!

// ✅ Correct: Use advanced API for labels
req := client.CreateDeviceRequest{
    Metadata: fabrica.Metadata{Name: "switch-01"},
    Spec: spec,
    Labels: map[string]string{"environment": "production"},
}
device, err := client.CreateDevice(ctx, req)
```

### Issue: UID not being preserved on update

**Cause:** This is expected behavior - UIDs are immutable

**Explanation:**
```go
// The server always preserves the UID
// Whether you use simple or advanced API, the UID never changes
device, err := client.UpdateDeviceSimple(ctx, originalUID, spec)
// device.Metadata.UID == originalUID (always)
```

## Summary

- **Simple API**: Use for 90% of operations - minimal boilerplate, just name + spec
- **Advanced API**: Use when you need labels, annotations, or full metadata control
- **No breaking changes**: Both APIs coexist, migrate gradually
- **Best practice**: Start simple, switch to advanced only when needed

## Next Steps

- **Try the examples**: See [examples/01-basic-crud](../../examples/01-basic-crud/) for working code
- **Learn about labels**: Use labels for organization and filtering
- **Explore advanced features**: API versioning, authentication, custom middleware

## Additional Resources

- [Getting Started Guide](getting-started.md) - Complete walkthrough
- [Resource Model Guide](resource-model.md) - Understanding Spec/Status
- [API Versioning](versioning.md) - Version negotiation
- [Validation Guide](validation.md) - Request validation
