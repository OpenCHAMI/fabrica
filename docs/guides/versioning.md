<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Versioning Guide

> Multi-version schema support with automatic conversion and HTTP negotiation.

## Table of Contents

- [Overview](#overview)
- [Why Versioning](#why-versioning)
- [Version Registration](#version-registration)
- [Conversion Patterns](#conversion-patterns)
- [HTTP Negotiation](#http-negotiation)
- [Migration Strategies](#migration-strategies)
- [Best Practices](#best-practices)

## Overview

Fabrica supports multiple schema versions for each resource type:

- **Simultaneous versions** - Run v1, v2, v3 at the same time
- **Automatic conversion** - Transparent version transformation
- **HTTP negotiation** - Client requests preferred version
- **Gradual migration** - Update clients at their own pace

## Why Versioning

### The Challenge

APIs evolve, but clients don't upgrade immediately:

```
Day 1: Launch v1 API
Day 30: Need new fields
Day 60: Launch v2 API
Day 90: Still have v1 clients!
```

### The Solution

Support both versions simultaneously:

```
Client A (v1) ──┐
               ├─→ Server (v1 + v2) ── Storage (v2)
Client B (v2) ──┘
```

Server automatically converts between versions.

### Use Cases

1. **Backward Compatibility**
   - Old clients continue working
   - No forced upgrades
   - Gradual migration

2. **Forward Compatibility**
   - New features without breaking changes
   - Beta versions for testing
   - Alpha versions for experiments

3. **Deprecation**
   - Mark versions as deprecated
   - Provide migration period
   - Remove when safe

## Version Registration

### Single Version (Default)

```go
// Just register the resource
generator.RegisterResource(&device.Device{})
```

This creates v1 by default.

### Multiple Versions

```go
// Register base resource (v1)
generator.RegisterResource(&device.DeviceV1{})

// Add v2beta1
generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v2beta1",
    IsDefault:  false,
    Stability:  "beta",
    Deprecated: false,
    SpecType:   "device.DeviceV2Spec",
    StatusType: "device.DeviceV2Status",
    TypeName:   "*device.DeviceV2",
    Package:    "github.com/yourapp/pkg/resources/device",
})

// Add v2 (stable)
generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v2",
    IsDefault:  true, // Make this the default
    Stability:  "stable",
    Deprecated: false,
    SpecType:   "device.DeviceV2Spec",
    StatusType: "device.DeviceV2Status",
    TypeName:   "*device.DeviceV2",
    Package:    "github.com/yourapp/pkg/resources/device",
})
```

### Version Format

Follow semantic versioning:

```
v1              - Stable version 1
v2              - Stable version 2
v2beta1         - Beta release of v2
v2beta2         - Second beta of v2
v3alpha1        - Alpha release of v3
```

## Conversion Patterns

### Define Version Structs

**v1 (stable):**
```go
// pkg/resources/device/v1/device.go
package v1

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Username string `json:"username"` // Flat auth
    Password string `json:"password"`
}
```

**v2 (with structured auth):**
```go
// pkg/resources/device/v2/device.go
package v2

type Device struct {
    resource.Resource
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status,omitempty"`
}

type DeviceSpec struct {
    Name     string `json:"name"`
    Location string `json:"location"`
    Auth     AuthConfig `json:"auth"` // Structured auth
}

type AuthConfig struct {
    Type     string `json:"type"` // "basic", "oauth", "cert"
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`
    Token    string `json:"token,omitempty"`
}
```

### Implement Converter

```go
// pkg/resources/device/converter.go
package device

type DeviceConverter struct{}

func (c *DeviceConverter) CanConvert(from, to string) bool {
    validPairs := map[string][]string{
        "v1": {"v2"},
        "v2": {"v1"},
    }
    targets, ok := validPairs[from]
    if !ok {
        return false
    }
    for _, t := range targets {
        if t == to {
            return true
        }
    }
    return false
}

func (c *DeviceConverter) Convert(resource interface{}, from, to string) (interface{}, error) {
    if from == "v1" && to == "v2" {
        return c.v1ToV2(resource)
    }
    if from == "v2" && to == "v1" {
        return c.v2ToV1(resource)
    }
    return nil, fmt.Errorf("conversion not supported: %s -> %s", from, to)
}

func (c *DeviceConverter) v1ToV2(resource interface{}) (*v2.Device, error) {
    v1Device := resource.(*v1.Device)

    v2Device := &v2.Device{
        Resource: v1Device.Resource,
        Spec: v2.DeviceSpec{
            Name:     v1Device.Spec.Name,
            Location: v1Device.Spec.Location,
            Auth: v2.AuthConfig{
                Type:     "basic",
                Username: v1Device.Spec.Username,
                Password: v1Device.Spec.Password,
            },
        },
        Status: v2.DeviceStatus(v1Device.Status),
    }

    return v2Device, nil
}

func (c *DeviceConverter) v2ToV1(resource interface{}) (*v1.Device, error) {
    v2Device := resource.(*v2.Device)

    // Warning: Lossy conversion if auth type is not "basic"
    username, password := "", ""
    if v2Device.Spec.Auth.Type == "basic" {
        username = v2Device.Spec.Auth.Username
        password = v2Device.Spec.Auth.Password
    }

    v1Device := &v1.Device{
        Resource: v2Device.Resource,
        Spec: v1.DeviceSpec{
            Name:     v2Device.Spec.Name,
            Location: v2Device.Spec.Location,
            Username: username,
            Password: password,
        },
        Status: v1.DeviceStatus(v2Device.Status),
    }

    return v1Device, nil
}
```

### Register Converter

```go
versioning.GlobalVersionRegistry.RegisterVersion("Device", "v1", versioning.ResourceTypeInfo{
    Type:        reflect.TypeOf(&v1.Device{}),
    Constructor: func() interface{} { return &v1.Device{} },
    Converter:   &DeviceConverter{},
    Metadata:    schemaV1,
})

versioning.GlobalVersionRegistry.RegisterVersion("Device", "v2", versioning.ResourceTypeInfo{
    Type:        reflect.TypeOf(&v2.Device{}),
    Constructor: func() interface{} { return &v2.Device{} },
    Converter:   &DeviceConverter{},
    Metadata:    schemaV2,
})
```

## HTTP Negotiation

### Client Requests Version

**Request:**
```http
GET /devices/dev-123
Accept: application/json;version=v1
```

**Flow:**
1. Client requests v1
2. Server loads from storage (might be v2)
3. Converter transforms v2 → v1
4. Response in v1 format

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json
X-Schema-Version: v1

{
  "apiVersion": "v1",
  "kind": "Device",
  "spec": {
    "username": "admin",
    "password": "secret"
  }
}
```

### Using curl

```bash
# Request v1
curl -H "Accept: application/json;version=v1" \
  http://localhost:8080/devices/dev-123

# Request v2
curl -H "Accept: application/json;version=v2" \
  http://localhost:8080/devices/dev-123

# Request default version (omit version)
curl http://localhost:8080/devices/dev-123
```

### Client Library

```go
// Create client with version
client := NewDeviceClient("http://localhost:8080")
client.SetVersion("v1")

// Get device in v1 format
device, err := client.GetDevice(ctx, "dev-123")
```

## Migration Strategies

### Strategy 1: Big Bang (Not Recommended)

```
Day 1: Launch v2, deprecate v1
Day 30: Remove v1
```

**Pros:** Simple
**Cons:** Breaks clients, forces immediate migration

### Strategy 2: Gradual Migration (Recommended)

```
Day 1: Launch v2beta1 for testing
Day 30: Promote to v2 stable
Day 60: Mark v1 as deprecated
Day 180: Remove v1 support
```

**Pros:** Smooth transition, no breakage
**Cons:** More work

### Strategy 3: Parallel Versions

```
Support v1, v2, v3 indefinitely
```

**Pros:** Maximum compatibility
**Cons:** Maintenance burden

### Migration Example

**Phase 1: Launch Beta (Day 1)**
```go
generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v2beta1",
    Stability:  "beta",
    Deprecated: false,
})
```

Client communication:
> "v2beta1 is available for testing. Please try it and report issues."

**Phase 2: Promote to Stable (Day 30)**
```go
generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v2",
    IsDefault:  true,
    Stability:  "stable",
    Deprecated: false,
})
```

Client communication:
> "v2 is now stable and the default. v1 is still supported."

**Phase 3: Deprecate Old Version (Day 60)**
```go
generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v1",
    Stability:  "stable",
    Deprecated: true, // Mark as deprecated
})
```

Client communication:
> "v1 is deprecated. Please migrate to v2 by Day 180."

**Phase 4: Remove Old Version (Day 180)**
```go
// Simply don't register v1 anymore
```

Client communication:
> "v1 has been removed. All clients must use v2."

## Best Practices

### Version Design

**DO:**
```go
✅ Use semantic versioning (v1, v2, v3)
✅ Mark stability (alpha, beta, stable)
✅ Provide bidirectional conversion
✅ Document breaking changes
✅ Give deprecation warnings

generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "v2beta1",
    Stability:  "beta",
    Deprecated: false,
})
```

**DON'T:**
```go
❌ Use arbitrary version strings
❌ Break existing versions
❌ Skip beta/alpha for major changes
❌ Remove versions without warning

generator.AddResourceVersion("Device", codegen.SchemaVersion{
    Version:    "new-version", // Bad!
})
```

### Conversion

**DO:**
```go
✅ Handle all field mappings
✅ Document lossy conversions
✅ Provide default values
✅ Test all conversion paths
✅ Log conversion warnings

func (c *Converter) v2ToV1(v2 *DeviceV2) (*DeviceV1, error) {
    if v2.Spec.Auth.Type != "basic" {
        log.Warn("Non-basic auth will be lost in v1 conversion")
    }
    // Convert...
}
```

### Migration

**DO:**
```go
✅ Communicate early and often
✅ Provide migration guides
✅ Support multiple versions during transition
✅ Give adequate deprecation period (3-6 months)
✅ Monitor version usage
```

## Summary

Fabrica versioning provides:

- 🔄 **Multiple versions** - Run v1, v2, v3 simultaneously
- 🔀 **Automatic conversion** - Transparent transformations
- 📡 **HTTP negotiation** - Client chooses version
- 🚀 **Smooth migrations** - Update at your own pace

**Next Steps:**
- Implement converters for your resources
- Test version conversion thoroughly
- Plan your deprecation strategy
- Monitor version usage metrics

---

**Questions?** [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
