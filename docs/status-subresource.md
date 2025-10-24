<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Status Subresource Guide

**Status:** Implemented ✅
**Version:** 0.3.0+
**Pattern:** Kubernetes-inspired

## Overview

Fabrica-generated microservices support Kubernetes-style status subresources, providing clear separation between desired state (spec) and observed state (status). This prevents conflicts between user updates and system controllers.

## Quick Start

```bash
# Generate a project (status subresource is automatic)
fabrica init my-api --module github.com/example/my-api
fabrica add resource Device
fabrica generate

# Build and run
go mod tidy
go run cmd/server/main.go
```

**Generated Endpoints:**
```
PUT    /devices/{uid}        → Update spec (user-facing)
PUT    /devices/{uid}/status → Update status (system-facing)
PATCH  /devices/{uid}/status → Patch status
```

## Architecture

### The Problem

Without status subresources, a single endpoint updates both spec and status:

```go
// ❌ Old pattern - conflicts possible
PUT /devices/{uid}
{
  "spec": {"location": "dc2"},     // User wants this
  "status": {"phase": "Ready"}     // Controller wants this
}
```

**Issues:**
- Users can accidentally overwrite system-managed status
- Controllers can accidentally overwrite user-defined spec
- No clear authorization boundary
- Race conditions between user and controller updates

### The Solution

Status subresources provide separate endpoints:

```go
// ✅ New pattern - no conflicts
PUT /devices/{uid}          // Users update spec only
{
  "spec": {"location": "dc2"}
}

PUT /devices/{uid}/status   // Controllers update status only
{
  "status": {"phase": "Ready", "health": "Healthy"}
}
```

**Benefits:**
- ✅ Clear separation of concerns
- ✅ No accidental overwrites
- ✅ Fine-grained authorization (RBAC)
- ✅ Kubernetes-familiar pattern
- ✅ Concurrent updates work correctly

## API Usage

### User Operations (Spec)

Update desired state:

```bash
# Update device location (spec)
curl -X PUT http://localhost:8080/devices/dev-123 \
  -H "Content-Type: application/json" \
  -d '{
    "spec": {
      "name": "sensor-01",
      "location": "datacenter-2",
      "model": "TempSensor-Pro"
    }
  }'
```

Patch specific fields:

```bash
# Patch just the location
curl -X PATCH http://localhost:8080/devices/dev-123 \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "spec": {
      "location": "datacenter-3"
    }
  }'
```

### Controller Operations (Status)

Update observed state:

```bash
# Update device status
curl -X PUT http://localhost:8080/devices/dev-123/status \
  -H "Content-Type: application/json" \
  -d '{
    "status": {
      "phase": "Ready",
      "health": "Healthy",
      "lastSeen": "2025-10-24T16:00:00Z",
      "conditions": [
        {
          "type": "Ready",
          "status": "True",
          "reason": "DeviceOnline",
          "message": "Device is operational"
        }
      ]
    }
  }'
```

Patch status fields:

```bash
# Patch just the health field
curl -X PATCH http://localhost:8080/devices/dev-123/status \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "status": {
      "health": "Degraded"
    }
  }'
```

## Client Library Usage

### Updating Spec

```go
import (
    "context"
    "github.com/example/my-api/pkg/client"
    "github.com/example/my-api/pkg/resources/device"
)

func updateDeviceLocation(c *client.Client, uid string) error {
    ctx := context.Background()

    // Update spec only
    spec := device.DeviceSpec{
        Name:     "sensor-01",
        Location: "datacenter-2",
        Model:    "TempSensor-Pro",
    }

    req := client.UpdateDeviceRequest{
        Name:       "sensor-01",
        DeviceSpec: spec,
    }

    device, err := c.UpdateDevice(ctx, uid, req)
    if err != nil {
        return err
    }

    fmt.Printf("Updated device spec: %s\n", device.Spec.Location)
    return nil
}
```

### Updating Status

```go
func updateDeviceStatus(c *client.Client, uid string) error {
    ctx := context.Background()

    // Update status only
    status := device.DeviceStatus{
        Phase:   "Ready",
        Health:  "Healthy",
        Message: "Device is operational",
    }

    device, err := c.UpdateDeviceStatus(ctx, uid, status)
    if err != nil {
        return err
    }

    fmt.Printf("Updated device status: %s\n", device.Status.Phase)
    return nil
}
```

### Patching Status

```go
func patchDeviceHealth(c *client.Client, uid string) error {
    ctx := context.Background()

    // Patch just the health field
    patch := []byte(`{"health": "Degraded"}`)

    device, err := c.PatchDeviceStatus(ctx, uid, patch)
    if err != nil {
        return err
    }

    fmt.Printf("Patched device health: %s\n", device.Status.Health)
    return nil
}
```

## Reconciler Pattern

Reconcilers automatically use status-only updates through the `BaseReconciler.UpdateStatus()` method.

### Example Reconciler

```go
package reconcilers

import (
    "context"
    "time"

    "github.com/alexlovelltroy/fabrica/pkg/reconcile"
    "github.com/example/my-api/pkg/resources/device"
)

type DeviceReconciler struct {
    reconcile.BaseReconciler
}

func (r *DeviceReconciler) reconcileDevice(ctx context.Context, dev *device.Device) error {
    // 1. Observe actual state (e.g., check if device is online)
    isOnline := r.checkDeviceOnline(dev)

    // 2. Update status based on observation
    if isOnline {
        dev.Status.Phase = "Ready"
        dev.Status.Health = "Healthy"
        dev.Status.LastSeen = time.Now().Format(time.RFC3339)
    } else {
        dev.Status.Phase = "Offline"
        dev.Status.Health = "Unhealthy"
    }

    // 3. UpdateStatus automatically:
    //    - Loads fresh copy from storage
    //    - Preserves any concurrent spec changes
    //    - Only updates status fields
    if err := r.UpdateStatus(ctx, dev); err != nil {
        return err
    }

    return nil
}

func (r *DeviceReconciler) checkDeviceOnline(dev *device.Device) bool {
    // Implement actual device health check
    return true
}
```

### How UpdateStatus Works

The `BaseReconciler.UpdateStatus()` method ensures spec safety:

```go
// From pkg/reconcile/reconciler.go
func (r *BaseReconciler) UpdateStatus(ctx context.Context, resource interface{}) error {
    // 1. Extract resource UID
    uid := resource.GetUID()
    kind := resource.GetKind()

    // 2. Load FRESH copy from storage (gets any concurrent spec updates)
    current, err := r.Client.Get(ctx, kind, uid)
    if err != nil {
        return err
    }

    // 3. Copy status from reconciled resource to fresh resource
    //    (preserves fresh spec, applies reconciled status)
    current.Status = resource.Status
    current.Touch()

    // 4. Save (spec is preserved, status is updated)
    return r.Client.Update(ctx, current)
}
```

## Resource Definition

Define resources with separate Spec and Status:

```go
package device

import "github.com/alexlovelltroy/fabrica/pkg/resource"

// Device represents a monitored device
type Device struct {
    resource.Resource `json:",inline"`
    Spec              DeviceSpec   `json:"spec"`
    Status            DeviceStatus `json:"status,omitempty"`
}

// DeviceSpec defines desired state (user-managed)
type DeviceSpec struct {
    Name        string            `json:"name" validate:"required"`
    Location    string            `json:"location" validate:"required"`
    Model       string            `json:"model,omitempty"`
    Config      map[string]string `json:"config,omitempty"`
}

// DeviceStatus defines observed state (system-managed)
type DeviceStatus struct {
    // Phase represents the lifecycle phase
    Phase   string `json:"phase,omitempty"` // Pending, Ready, Offline, Error

    // Health represents operational health
    Health  string `json:"health,omitempty"` // Healthy, Degraded, Unhealthy

    // Message provides human-readable status
    Message string `json:"message,omitempty"`

    // LastSeen is when the device was last contacted
    LastSeen string `json:"lastSeen,omitempty"`

    // Conditions track specific state transitions
    Conditions []resource.Condition `json:"conditions,omitempty"`
}
```

## Authorization

### Separate Permissions

Status subresources support fine-grained authorization:

```go
// In your policy implementation
type DevicePolicy struct {
    enforcer *casbin.Enforcer
}

// Users can update spec
func (p *DevicePolicy) CanUpdate(ctx context.Context, auth interface{}, r *http.Request, uid string) PolicyDecision {
    user := getUserFromAuth(auth)

    if p.enforcer.HasPermission(user, "devices", "update") {
        return PolicyDecision{Allowed: true}
    }

    return PolicyDecision{
        Allowed: false,
        Reason:  "requires devices:update permission",
    }
}

// Controllers can update status (optional interface)
func (p *DevicePolicy) CanUpdateStatus(ctx context.Context, auth interface{}, r *http.Request, uid string) PolicyDecision {
    user := getUserFromAuth(auth)

    // Only controllers can update status
    if p.enforcer.HasRoleForUser(user, "controller") {
        return PolicyDecision{Allowed: true}
    }

    return PolicyDecision{
        Allowed: false,
        Reason:  "requires controller role",
    }
}
```

### Casbin Policy Example

```csv
# policies.csv

# Users can update device specs
p, user, devices, update, allow
p, user, devices, update_status, deny

# Controllers can update device status
p, controller, devices, update, deny
p, controller, devices, update_status, allow

# Admins can do both
p, admin, devices, update, allow
p, admin, devices, update_status, allow

# Role assignments
g, alice, user
g, device-controller, controller
g, bob, admin
```

## Events

Status updates publish lifecycle events with distinguishing metadata:

### Status Update Event

```json
{
  "specversion": "1.0",
  "type": "io.fabrica.device.updated",
  "source": "fabrica-api/resources/Device/dev-123",
  "id": "evt-abc123",
  "time": "2025-10-24T16:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "action": "updated",
    "resourceKind": "Device",
    "resourceUID": "dev-123",
    "resourceName": "sensor-01",
    "resource": { ... },
    "metadata": {
      "updatedAt": "2025-10-24T16:00:00Z",
      "updateType": "status"
    }
  }
}
```

### Spec Update Event

```json
{
  "type": "io.fabrica.device.updated",
  "data": {
    "metadata": {
      "updatedAt": "2025-10-24T16:00:00Z"
      // No updateType field (or updateType: "spec")
    }
  }
}
```

### Subscribing to Events

```go
// Subscribe to all device updates
eventBus.Subscribe("io.fabrica.device.updated", func(ctx context.Context, event events.Event) error {
    var data events.ResourceChangeData
    event.DataAs(&data)

    // Check if this is a status update
    if updateType, ok := data.Metadata["updateType"].(string); ok && updateType == "status" {
        fmt.Printf("Status updated for device %s\n", data.ResourceUID)
    } else {
        fmt.Printf("Spec updated for device %s\n", data.ResourceUID)
    }

    return nil
})
```

## Testing

### Test Spec/Status Separation

```go
func TestStatusSubresource(t *testing.T) {
    // Create device
    device := createDevice(t, DeviceSpec{
        Name:     "sensor-01",
        Location: "dc1",
    })

    // User updates spec
    device.Spec.Location = "dc2"
    updateDevice(t, device.UID, device.Spec)

    // Controller updates status
    status := DeviceStatus{
        Phase:  "Ready",
        Health: "Healthy",
    }
    updateDeviceStatus(t, device.UID, status)

    // Verify both are updated
    final := getDevice(t, device.UID)
    assert.Equal(t, "dc2", final.Spec.Location)
    assert.Equal(t, "Ready", final.Status.Phase)
}
```

## Best Practices

### ✅ DO

1. **Use status for observed state only**
   ```go
   status.Phase = "Ready"        // ✅ Observed
   status.Health = "Healthy"     // ✅ Observed
   status.LastSeen = time.Now()  // ✅ Observed
   ```

2. **Update status in reconcilers**
   ```go
   dev.Status.Phase = "Ready"
   r.UpdateStatus(ctx, dev)  // ✅ Status-only update
   ```

3. **Use conditions for details**
   ```go
   resource.SetResourceCondition(ctx, dev,
       "Ready", "True", "DeviceOnline", "Device is operational")
   ```

### ❌ DON'T

1. **Don't put computed values in spec**
   ```go
   spec.CalculatedField = "value"  // ❌ Use status
   ```

2. **Don't update spec from reconcilers**
   ```go
   dev.Spec.Location = "new"  // ❌ Spec is user-defined
   ```

3. **Don't mix spec and status in updates**
   ```go
   // ❌ Old pattern
   PUT /devices/{uid}
   {
     "spec": {...},
     "status": {...}
   }
   ```

## Troubleshooting

### Status Update Doesn't Work

**Problem:** Status update returns 404 or 405

**Solution:** Verify route is registered:
```bash
# Check generated routes
grep "status" cmd/server/routes_generated.go
```

Should show:
```go
r.Route("/status", func(r chi.Router) {
    r.Put("/", UpdateDeviceStatus)
    r.Patch("/", PatchDeviceStatus)
})
```

### Spec Gets Overwritten

**Problem:** Reconciler overwrites spec changes

**Solution:** Use `BaseReconciler.UpdateStatus()` which loads fresh resource:

```go
// ❌ Don't do this
r.Client.Update(ctx, dev)  // Overwrites everything

// ✅ Do this instead
r.UpdateStatus(ctx, dev)   // Only updates status
```

### Authorization Fails

**Problem:** Controllers can't update status

**Solution:** Implement `StatusPolicy` interface:

```go
type DevicePolicy struct {
    // ... existing fields
}

// Add this method
func (p *DevicePolicy) CanUpdateStatus(ctx context.Context, auth interface{}, r *http.Request, uid string) PolicyDecision {
    // Check if user has controller role
    user := getUserFromAuth(auth)
    if isController(user) {
        return PolicyDecision{Allowed: true}
    }
    return PolicyDecision{Allowed: false, Reason: "not a controller"}
}
```

## Migration from Old Pattern

If you have existing code that updates both spec and status:

### Before
```go
// Old pattern - updates everything
dev.Spec.Location = "dc2"
dev.Status.Phase = "Ready"
storage.SaveDevice(ctx, dev)
```

### After
```go
// New pattern - separate updates

// User updates (spec)
dev.Spec.Location = "dc2"
storage.SaveDevice(ctx, dev)

// Controller updates (status) - in reconciler
dev.Status.Phase = "Ready"
r.UpdateStatus(ctx, dev)  // Loads fresh, preserves spec
```

## OpenAPI Documentation

Generated OpenAPI specs include status subresource operations:

```yaml
paths:
  /devices/{uid}/status:
    put:
      summary: Update device status
      description: |
        Updates only the status portion of a device. This endpoint is intended
        for controllers, reconcilers, and monitoring systems.
      parameters:
        - name: uid
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DeviceStatus'
      responses:
        '200':
          description: Status updated successfully
        '403':
          description: Forbidden - requires update_status permission
```

## Further Reading

- [Resource Model Guide](resource-model.md) - Resource structure and metadata
- [Events Guide](events.md) - Event system and CloudEvents
- [Reconciliation Guide](reconciliation.md) - Building reconcilers
- [Kubernetes Status Subresources](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource)

---

**Implementation Status:** ✅ Complete
**Version:** Fabrica 0.3.0+
**Questions?** [GitHub Discussions](https://github.com/alexlovelltroy/fabrica/discussions)
