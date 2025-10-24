<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Resource Model Guide

> Understanding Fabrica's resource structure, UID generation, labels, annotations, and lifecycle.

## Table of Contents

- [Overview](#overview)
- [Resource Structure](#resource-structure)
- [UID Generation](#uid-generation)
- [Metadata](#metadata)
- [Labels and Annotations](#labels-and-annotations)
- [Resource Lifecycle](#resource-lifecycle)
- [Best Practices](#best-practices)

## Overview

Fabrica follows the Kubernetes resource pattern. All resources embed a common `Resource` struct:

```go
type Device struct {
    resource.Resource `json:",inline"`
    Spec              DeviceSpec   `json:"spec"`
    Status            DeviceStatus `json:"status"`
}

// The embedded Resource struct provides:
type Resource struct {
    APIVersion    string    `json:"apiVersion"`    // "v1"
    Kind          string    `json:"kind"`          // "Device", "User", "Product"
    SchemaVersion string    `json:"schemaVersion"` // Schema migration support
    Metadata      Metadata  `json:"metadata"`      // Name, UID, labels, annotations, timestamps
}
```

## Resource Structure

### APIVersion

The version of the API that defines this resource.

```go
APIVersion: "v1"        // Stable version
APIVersion: "v2beta1"   // Beta version
APIVersion: "v3alpha1"  // Alpha version
```

**Usage:**
- Enables multi-version support
- Allows gradual API evolution
- Supports backward compatibility

### Kind

The type of resource.

```go
Kind: "Device"      // IoT device
Kind: "User"        // User account
Kind: "Product"     // Product catalog item
```

**Convention:** Use PascalCase singular nouns.

### Metadata

Standard metadata for all resources:

```go
type Metadata struct {
    UID         string            // Unique identifier
    Name        string            // Human-readable name
    Labels      map[string]string // Queryable key-value pairs
    Annotations map[string]string // Non-queryable metadata
    CreatedAt   time.Time         // Creation timestamp
    UpdatedAt   time.Time         // Last update timestamp
}
```

### Spec (Desired State)

Your custom specification - what you want:

```go
type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Model    string `json:"model"`
    Config   map[string]string `json:"config,omitempty"`
}
```

**Principles:**
- Immutable desired state
- User-provided values
- What should exist

### Status (Observed State)

Your custom status - what actually is:

```go
type DeviceStatus struct {
    Active       bool   `json:"active"`
    LastSeen     string `json:"lastSeen,omitempty"`
    IPAddress    string `json:"ipAddress,omitempty"`
    Health       string `json:"health,omitempty"`
    Conditions   []Condition `json:"conditions,omitempty"`
}
```

**Principles:**
- Mutable observed state
- System-provided values
- What actually exists

## UID Generation

Fabrica uses structured UIDs instead of UUIDs for better readability and debugging.

### Format

```
<prefix>-<random-hex>

Examples:
  dev-1a2b3c4d    (Device)
  usr-9f8e7d6c    (User)
  prd-5a4b3c2d    (Product)
```

### Register a Prefix

```go
func init() {
    resource.RegisterResourcePrefix("Device", "dev")
    resource.RegisterResourcePrefix("User", "usr")
    resource.RegisterResourcePrefix("Product", "prd")
}
```

### Generate UIDs

**Automatic (recommended):**
```go
// Framework generates UID automatically on create
device := &Device{
    Spec: DeviceSpec{Name: "Sensor 1"},
}
// UID will be generated: dev-1a2b3c4d
```

**Manual:**
```go
uid, err := resource.GenerateUIDForResource("Device")
// Returns: "dev-1a2b3c4d"

// Custom length
uid, err := resource.GenerateUIDWithLength("dev", 12)
// Returns: "dev-1a2b3c4d5e6f"
```

### UID Utilities

```go
// Parse UID
prefix, random, err := resource.ParseUID("dev-1a2b3c4d")
// prefix = "dev", random = "1a2b3c4d"

// Validate UID
valid := resource.IsValidUID("dev-1a2b3c4d") // true

// Get resource type from UID
kind, err := resource.GetResourceTypeFromUID("dev-1a2b3c4d")
// Returns: "Device"
```

## Metadata

### Name

Human-readable identifier:

```go
device.Metadata.Name = "temperature-sensor-01"
```

**Best Practices:**
- Use lowercase with hyphens
- Include context (location, purpose, number)
- Make it meaningful for humans

### UID

System-generated unique identifier:

```go
device.Metadata.UID = "dev-1a2b3c4d"
```

**Best Practices:**
- Never modify after creation
- Use for all internal references
- Use for API paths (`/devices/dev-1a2b3c4d`)

### Timestamps

Automatically managed:

```go
device.Metadata.CreatedAt = time.Now()  // On create
device.Metadata.UpdatedAt = time.Now()  // On every update
```

**Utilities:**
```go
// How old is the resource?
age := device.Age()

// When was it last updated?
lastUpdate := device.LastUpdated()

// Mark as updated
device.Touch()
```

## Labels and Annotations

### Labels

Queryable key-value pairs for selection and grouping:

```go
device.SetLabel("environment", "production")
device.SetLabel("location", "datacenter-01")
device.SetLabel("team", "platform")
```

**Use labels for:**
- Filtering (`?label=environment=production`)
- Grouping (all devices in a location)
- Selection (which resources to process)
- Organization (teams, projects, environments)

**Example queries:**
```go
// Get label
env, exists := device.GetLabel("environment")

// Check if label exists
if device.HasLabel("critical") {
    // Handle critical device
}

// Match multiple labels
selector := map[string]string{
    "environment": "production",
    "location": "datacenter-01",
}
if device.MatchesLabels(selector) {
    // Device matches criteria
}

// Get all labels
labels := device.GetLabels()
for key, value := range labels {
    fmt.Printf("%s=%s\n", key, value)
}
```

### Annotations

Non-queryable metadata for additional context:

```go
device.SetAnnotation("description", "Primary temperature sensor for cold storage")
device.SetAnnotation("contact.email", "ops@example.com")
device.SetAnnotation("purchased.date", "2024-01-15")
device.SetAnnotation("warranty.expires", "2027-01-15")
```

**Use annotations for:**
- Descriptions and documentation
- Contact information
- External references
- Configuration hints
- Build/deployment metadata

**Example usage:**
```go
// Get annotation
desc, exists := device.GetAnnotation("description")

// Get all annotations
annotations := device.GetAnnotations()

// Remove annotation
device.RemoveAnnotation("old-field")
```

### Labels vs. Annotations

| Use Case | Use Labels | Use Annotations |
|----------|------------|-----------------|
| Filtering/querying | ✅ | ❌ |
| Grouping resources | ✅ | ❌ |
| Human documentation | ❌ | ✅ |
| External references | ❌ | ✅ |
| Configuration data | ❌ | ✅ |

## Resource Lifecycle

### 1. Creation

```go
// Create resource
device := &Device{
    Resource: resource.Resource{
        APIVersion: "v1",
        Kind: "Device",
    },
    Spec: DeviceSpec{
        Name: "Temperature Sensor",
        Location: "Warehouse A",
    },
}

// Metadata is initialized
device.Metadata.Initialize("temp-sensor-01", uid)
device.SetLabel("location", "warehouse-a")

// Save to storage
storage.Save(ctx, device)
```

**What happens:**
1. UID generated: `dev-1a2b3c4d`
2. Timestamps set: `CreatedAt`, `UpdatedAt`
3. Persisted to storage

### 2. Reading

```go
// Get by UID
device, err := storage.Load(ctx, "dev-1a2b3c4d")

// List all
devices, err := storage.LoadAll(ctx)

// Filter by labels (application level)
productionDevices := []Device{}
for _, d := range devices {
    if d.Metadata.Labels["environment"] == "production" {
        productionDevices = append(productionDevices, d)
    }
}
```

### 3. Updating

```go
// Load resource
device, err := storage.Load(ctx, "dev-1a2b3c4d")

// Update spec (desired state)
device.Spec.Location = "Warehouse B"

// Update label
device.SetLabel("location", "warehouse-b")

// Update status (observed state)
device.Status.IPAddress = "192.168.1.100"
device.Status.LastSeen = time.Now().Format(time.RFC3339)

// Mark as updated
device.Touch()

// Save
storage.Save(ctx, device)
```

**What happens:**
1. `UpdatedAt` timestamp refreshed
2. Changes persisted to storage

### 4. Deletion

```go
// Delete by UID
err := storage.Delete(ctx, "dev-1a2b3c4d")
```

**What happens:**
1. Resource removed from storage
2. No soft-delete by default (implement if needed)

## Best Practices

### Resource Definition

**DO:**
```go
✅ Embed resource.Resource
✅ Use json tags
✅ Separate Spec and Status
✅ Register UID prefix in init()
✅ Add validation methods

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status,omitempty"`
}

func init() {
    resource.RegisterResourcePrefix("Device", "dev")
}

func (d *Device) Validate() error {
    if d.Spec.Name == "" {
        return fmt.Errorf("name required")
    }
    return nil
}
```

**DON'T:**
```go
❌ Mix Spec and Status fields
❌ Forget json tags
❌ Use UUID for UID
❌ Store computed values in Spec

type BadDevice struct {
    Name string  // Should be in Spec
    Online bool  // Should be in Status
}
```

### Labels

**DO:**
```go
✅ Use for queryable attributes
✅ Use lowercase with hyphens
✅ Keep values simple
✅ Use consistent naming

device.SetLabel("environment", "production")
device.SetLabel("team", "platform")
device.SetLabel("location", "us-west-2")
```

**DON'T:**
```go
❌ Store large values
❌ Use for documentation
❌ Include sensitive data
❌ Use inconsistent formats

device.SetLabel("Description", "A very long description...")
device.SetLabel("api_key", "secret123")
```

### Annotations

**DO:**
```go
✅ Use for documentation
✅ Store external references
✅ Include context
✅ Use structured keys

device.SetAnnotation("description", "Primary sensor")
device.SetAnnotation("contact.email", "ops@example.com")
device.SetAnnotation("external.id", "EXT-12345")
device.SetAnnotation("docs.url", "https://docs.example.com/devices/temp")
```

### Status Fields

**DO:**
```go
✅ Include conditions
✅ Add health indicators
✅ Record timestamps
✅ Show actual state

type DeviceStatus struct {
    Online       bool        `json:"online"`
    LastSeen     string      `json:"lastSeen,omitempty"`
    Health       string      `json:"health,omitempty"`
    Conditions   []Condition `json:"conditions,omitempty"`
}
```

**Condition Pattern:**

Conditions follow Kubernetes conventions and automatically publish events when changed:

```go
import "github.com/alexlovelltroy/fabrica/pkg/resource"

type Condition struct {
    Type               string    `json:"type"`               // "Ready", "Healthy", "Available"
    Status             string    `json:"status"`             // "True", "False", "Unknown"
    LastTransitionTime time.Time `json:"lastTransitionTime"` // Auto-set when status changes
    Reason             string    `json:"reason,omitempty"`   // Machine-readable reason
    Message            string    `json:"message,omitempty"`  // Human-readable description
}

// Manual condition setting
device.Status.Conditions = []Condition{
    {
        Type:               "Ready",
        Status:             "True",
        Reason:             "DeviceOnline",
        Message:            "Device is ready to accept commands",
        LastTransitionTime: time.Now(),
    },
}

// Recommended: Use helper functions (publishes events automatically)
ctx := context.Background()
changed := resource.SetResourceCondition(ctx, device,
    "Ready", "True", "DeviceOnline", "Device is operational")

if changed {
    // CloudEvent published: "io.fabrica.condition.ready"
    log.Println("Ready condition changed")
}

// Common condition patterns
resource.SetResourceCondition(ctx, device, "Healthy", "True", "HealthCheckPassed", "All health checks passing")
resource.SetResourceCondition(ctx, device, "Available", "False", "Maintenance", "Device under maintenance")
resource.SetResourceCondition(ctx, device, "Connected", "Unknown", "NetworkTimeout", "Network connectivity uncertain")
```

**Condition Event Integration:**
- Conditions automatically publish CloudEvents when status changes
- Event type: `{prefix}.condition.{type}` (e.g., `io.fabrica.condition.ready`)
- Only publishes when status actually changes (not on every update)
- Includes full condition details and resource context
- See [Events Guide](events.md) for complete condition event documentation

## Summary

Fabrica resources provide:

- 📋 **Structured format** - Kubernetes-inspired pattern
- 🔑 **Unique IDs** - Structured UIDs for readability
- 🏷️ **Labels** - Queryable organization
- 📝 **Annotations** - Rich metadata
- ⏰ **Timestamps** - Automatic lifecycle tracking
- 📊 **Status** - Observed state tracking

**Next Steps:**
- Build resources in [Getting Started](getting-started.md)
- Generate code in [Code Generation](codegen.md)
- Store resources in [Storage Guide](storage.md)

---

**Questions?** [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
