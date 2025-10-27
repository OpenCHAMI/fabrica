<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Example 6: Status Subresource Pattern

**Time to complete:** ~15 minutes
**Difficulty:** Beginner
**Prerequisites:** Basic understanding of REST APIs

## What You'll Learn

This example demonstrates the **status subresource pattern** - a Kubernetes-inspired approach for separating desired state (spec) from observed state (status).

**Key Concepts:**
- Separate endpoints for spec vs status updates
- Preventing conflicts between users and controllers
- Reconciler patterns with status-only updates
- Authorization for different update types

## The Problem

Without status subresources, a single endpoint updates everything:

```bash
# ❌ Problem: Both users and controllers use the same endpoint
PUT /devices/dev-123
{
  "spec": {"location": "dc2"},    # User wants this
  "status": {"phase": "Ready"}    # Controller wants this
}
```

**Issues:**
- Users can accidentally overwrite controller-managed status
- Controllers can accidentally overwrite user-defined spec
- Race conditions when both update simultaneously
- No way to authorize separately

## The Solution

Status subresources provide separate endpoints:

```bash
# ✅ Solution: Separate endpoints
PUT /devices/dev-123          # Users update spec
{
  "spec": {"location": "dc2"}
}

PUT /devices/dev-123/status   # Controllers update status
{
  "status": {"phase": "Ready"}
}
```

## Quick Start

### Step 1: Initialize Project

```bash
# Status subresources are automatic in all projects
fabrica init device-manager \
  --module github.com/example/device-manager \
  --storage-type file

cd device-manager
```

### Step 2: Add Device Resource

```bash
fabrica add resource Device
```

Edit `pkg/resources/device/device.go`:

```go
package device

import "github.com/openchami/fabrica/pkg/resource"

type Device struct {
    resource.Resource `json:",inline"`
    Spec              DeviceSpec   `json:"spec"`
    Status            DeviceStatus `json:"status,omitempty"`
}

// DeviceSpec - User-defined desired state
type DeviceSpec struct {
    Location string `json:"location" validate:"required"`
    Model    string `json:"model,omitempty"`
}

// DeviceStatus - System-observed state
type DeviceStatus struct {
    Phase      string `json:"phase,omitempty"`       // Pending, Ready, Offline
    Health     string `json:"health,omitempty"`      // Healthy, Degraded, Unhealthy
    LastSeen   string `json:"lastSeen,omitempty"`    // RFC3339 timestamp
    Message    string `json:"message,omitempty"`     // Human-readable
    Conditions []resource.Condition `json:"conditions,omitempty"`
}

func init() {
    resource.RegisterResourcePrefix("Device", "dev")
}
```

### Step 3: Generate Code

```bash
fabrica generate
go mod tidy
```

**What gets generated:**
```
cmd/server/
├── device_handlers_generated.go    # Includes UpdateDeviceStatus, PatchDeviceStatus
└── routes_generated.go             # Routes /devices/{uid}/status

pkg/client/
└── client_generated.go             # Includes UpdateDeviceStatus(), PatchDeviceStatus()
```

### Step 4: Build and Run

```bash
go build -o device-server ./cmd/server
./device-server
```

## Testing the API

### Create a Device

```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sensor-01",
    "location": "datacenter-1",
    "model": "TempPro-2000"
  }'
```

Save the UID from the response (e.g., `dev-abc123`).

### User Updates Spec

```bash
# User changes device location and name
curl -X PUT http://localhost:8080/devices/dev-abc123 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Temperature Sensor",
    "location": "datacenter-2",
    "model": "TempPro-2000"
  }'

# Response shows updated spec, status unchanged
{
  "apiVersion": "v1",
  "kind": "Device",
  "metadata": {
    "name": "sensor-01",
    "uid": "dev-abc123",
    ...
  },
  "spec": {
    "location": "datacenter-2",  // ✅ Updated by user
    ...
  },
  "status": { // unchaged
  }
}
```

### Controller Updates Status

```bash
# Controller reports device is ready
curl -X PUT http://localhost:8080/devices/dev-abc123/status \
  -H "Content-Type: application/json" \
  -d '{
    "phase": "Ready",
    "health": "Healthy",
    "lastSeen": "2025-10-24T16:00:00Z",
    "message": "Device is operational"
  }'

# Response shows updated status, spec unchanged
{
  "spec": {
    "location": "datacenter-2"  // ✅ Unchanged
  },
  "status": {
    "phase": "Ready",           // ✅ Updated by controller
    "health": "Healthy",
    ...
  }
}
```

### Patch Status

```bash
# Update just the health field
curl -X PATCH http://localhost:8080/devices/dev-abc123/status \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "health": "Degraded"
  }'
```

## Using the Client Library

### Updating Spec

Create the following file in the `device-manager` root directory:

```go
package main

import (
    "context"
    "log"

    "github.com/example/device-manager/pkg/client"
    "github.com/example/device-manager/pkg/resources/device"
)

func main() {
    // Create client
    c, err := client.NewClient("http://localhost:8080", nil)
    if err != nil {
        log.Fatal(err)
    }

    // User updates device location (spec)
    spec := device.DeviceSpec{
        Location: "datacenter-3",
        Model:    "TempPro-2000",
    }

    req := client.UpdateDeviceRequest{
        DeviceSpec: spec,
    }

    // TODO: Update with real deviceID from database
    dev, err := c.UpdateDevice(context.Background(), "dev-abc123", req)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Updated location to: %s\n", dev.Spec.Location)
}
```

Run the file with: `go run client-spec.go`. Make sure that the device ID has been updated.

### Updating Status

Create the following file in the `device-manager` root directory:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/example/device-manager/pkg/client"
    "github.com/example/device-manager/pkg/resources/device"
)

func main() {
    c, err := client.NewClient("http://localhost:8080", nil)
    if err != nil {
        log.Fatal(err)
    }

    // Controller updates device status
    status := device.DeviceStatus{
        Phase:    "Ready",
        Health:   "Healthy",
        LastSeen: time.Now().Format(time.RFC3339),
        Message:  "All systems operational",
    }

    // TODO: Update with real device ID from database
    dev, err := c.UpdateDeviceStatus(context.Background(), "dev-abc123", status)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Updated status to: %s\n", dev.Status.Phase)
}
```

Run the file with: `go run client-status.go`. Make sure that the device ID has been updated.

## Building a Controller

Create a simple controller that monitors devices:

```go
// pkg/controller/device_controller.go
package controller

import (
    "context"
    "log"
    "time"

    "github.com/example/device-manager/pkg/client"
    "github.com/example/device-manager/pkg/resources/device"
)

type DeviceController struct {
    client *client.Client
}

func NewDeviceController(apiURL string) (*DeviceController, error) {
    c, err := client.NewClient(apiURL, nil)
    if err != nil {
        return nil, err
    }

    return &DeviceController{client: c}, nil
}

func (dc *DeviceController) Run(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            dc.reconcile(ctx)
        }
    }
}

func (dc *DeviceController) reconcile(ctx context.Context) {
    // List all devices
    devices, err := dc.client.GetDevices(ctx)
    if err != nil {
        log.Printf("Failed to list devices: %v", err)
        return
    }

    // Check each device
    for _, dev := range devices {
        isOnline := dc.checkDevice(dev)

        // Update status based on observation
        status := device.DeviceStatus{
            LastSeen: time.Now().Format(time.RFC3339),
        }

        if isOnline {
            status.Phase = "Ready"
            status.Health = "Healthy"
            status.Message = "Device is responding"
        } else {
            status.Phase = "Offline"
            status.Health = "Unhealthy"
            status.Message = "Device is not responding"
        }

        // Update status (preserves spec)
        _, err := dc.client.UpdateDeviceStatus(ctx, dev.Metadata.UID, status)
        if err != nil {
            log.Printf("Failed to update status for %s: %v", dev.Metadata.Name, err)
        }
    }
}

func (dc *DeviceController) checkDevice(dev device.Device) bool {
    // Implement actual device health check
    // For example: ping device, check API endpoint, etc.
    return true
}
```

### Running the Controller

```go
// cmd/controller/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/example/device-manager/pkg/controller"
)

func main() {
    // Create controller
    dc, err := controller.NewDeviceController("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }

    // Run controller
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        log.Println("Shutting down controller...")
        cancel()
    }()

    log.Println("Starting device controller...")
    dc.Run(ctx)
}
```
Now run with `go run main.go`. Ensure that the `device_controller.go` file is in `pkg/controller/`, while `main.go` is in `cmd/controller/`

## Key Takeaways

### ✅ DO

1. **Update spec from user-facing code**
   ```go
   c.UpdateDevice(ctx, uid, spec)
   ```

2. **Update status from controllers**
   ```go
   c.UpdateDeviceStatus(ctx, uid, status)
   ```

3. **Use status for observed state**
   ```go
   status.Phase = "Ready"
   status.Health = "Healthy"
   status.LastSeen = time.Now()
   ```

### ❌ DON'T

1. **Don't update status from users**
   ```go
   // ❌ User shouldn't set status
   status.Phase = "Ready"
   c.UpdateDevice(ctx, uid, device)
   ```

2. **Don't update spec from controllers**
   ```go
   // ❌ Controller shouldn't change spec
   dev.Spec.Location = "new"
   ```

3. **Don't mix spec and status updates**
   ```go
   // ❌ Old pattern
   PUT /devices/{uid}
   { "spec": {...}, "status": {...} }
   ```

## Comparison with Traditional APIs

| Traditional API | Status Subresource |
|----------------|-------------------|
| Single endpoint | Separate endpoints |
| Mixed concerns | Clear separation |
| Conflict-prone | Safe concurrent updates |
| Single permission | Fine-grained auth |
| Manual coordination | Built-in safety |

## Authorization Example

```go
// Different permissions for spec vs status
type DeviceAuthMiddleware struct {
    // Your auth implementation here
}

// Users can update spec
func (m *DeviceAuthMiddleware) CheckUpdatePermission(r *http.Request, uid string) bool {
    user := getUserFromRequest(r)
    return hasRole(user, "device-admin")
}

// Controllers can update status
func (m *DeviceAuthMiddleware) CheckStatusUpdatePermission(r *http.Request, uid string) bool {
    user := getUserFromRequest(r)
    return hasRole(user, "controller")
}
```

## Further Reading

- [Status Subresource Guide](../../docs/status-subresource.md) - Complete reference
- [Resource Model](../../docs/resource-model.md) - Resource structure
- [Reconciliation Guide](../../docs/reconciliation.md) - Building controllers
- [Authorization Guide](../../docs/policy.md) - Access control patterns

## Summary

Status subresources provide:
- ✅ **Clear separation** - Spec for users, status for controllers
- ✅ **No conflicts** - Concurrent updates work correctly
- ✅ **Kubernetes-familiar** - Industry-standard pattern
- ✅ **Automatic** - Generated by default in all Fabrica projects
- ✅ **Type-safe** - Full client library support

**Next:** Build event-driven systems with [Reconciliation Example](../04-rack-reconciliation/)
