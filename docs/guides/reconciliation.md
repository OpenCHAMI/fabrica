<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Reconciliation Framework

Fabrica provides a Kubernetes-style reconciliation framework for building declarative infrastructure management systems. The reconciliation pattern enables automatic convergence of actual state to desired state.

## Overview

The reconciliation system consists of:
- **Reconciler**: Interface for resource-specific reconciliation logic
- **Controller**: Manages reconciler lifecycle and work queue
- **WorkQueue**: Thread-safe queue with deduplication and rate limiting
- **Event Integration**: Automatic reconciliation triggered by events

## Core Concepts

### Declarative Management

Instead of imperative commands:
```go
// Imperative (bad)
device.Connect()
device.Configure()
device.Start()
```

Use declarative state:
```go
// Declarative (good)
device.Spec.Desired = "running"
device.Spec.Config = {...}

// Reconciler automatically:
// - Connects if not connected
// - Configures if config changed
// - Starts if not running
```

### Reconciliation Loop

```
┌─────────────────────────────────────┐
│   1. Event triggers reconciliation  │
│      (create/update/periodic)       │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   2. Load current resource state    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   3. Compare Spec vs Status         │
│      (desired vs actual)            │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   4. Take actions to converge       │
│      (make actual match desired)    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   5. Update resource Status         │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   6. Requeue if needed              │
│      (periodic check/retry)         │
└─────────────────────────────────────┘
```

## Quick Start

### 1. Define Your Resource

```go
type Device struct {
    Kind   string       `json:"kind"`
    UID    string       `json:"uid"`
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status"`
}

type DeviceSpec struct {
    Desired string `json:"desired"` // "running", "stopped"
    Config  Config `json:"config"`
}

type DeviceStatus struct {
    State    string `json:"state"`
    LastSeen string `json:"lastSeen"`
}

func (d *Device) GetKind() string { return d.Kind }
func (d *Device) GetUID() string  { return d.UID }
```

### 2. Create a Reconciler

```go
import "github.com/openchami/fabrica/pkg/reconcile"

type DeviceReconciler struct {
    reconcile.BaseReconciler
}

func (r *DeviceReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    device := resource.(*Device)

    // Compare desired vs actual state
    if device.Spec.Desired == "running" && device.Status.State != "running" {
        // Take action to start device
        if err := r.startDevice(device); err != nil {
            return reconcile.Result{}, err
        }

        // Update status
        device.Status.State = "running"
        r.UpdateStatus(ctx, device)

        // Emit event
        r.EmitEvent(ctx, "io.example.device.started", device)
    }

    // Requeue after 5 minutes for periodic check
    return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *DeviceReconciler) GetResourceKind() string {
    return "Device"
}
```

### 3. Set Up Controller

```go
import (
    "github.com/openchami/fabrica/pkg/events"
    "github.com/openchami/fabrica/pkg/reconcile"
    "github.com/openchami/fabrica/pkg/storage"
)

func main() {
    // Create event bus
    eventBus := events.NewInMemoryEventBus(1000, 10)
    eventBus.Start()
    defer eventBus.Close()

    // Create storage
    storage := storage.NewInMemoryStorage()

    // Create controller
    controller := reconcile.NewController(eventBus, storage)

    // Register reconciler
    reconciler := &DeviceReconciler{
        BaseReconciler: reconcile.BaseReconciler{
            EventBus: eventBus,
            Logger:   reconcile.NewDefaultLogger(),
        },
    }
    controller.RegisterReconciler(reconciler)

    // Start controller
    ctx := context.Background()
    controller.Start(ctx)
    defer controller.Stop()

    // Controller now reconciles devices automatically!
    select {}
}
```

## Reconciler Interface

### Required Methods

```go
type Reconciler interface {
    // Reconcile brings resource to desired state
    Reconcile(ctx context.Context, resource interface{}) (Result, error)

    // GetResourceKind returns the resource kind handled
    GetResourceKind() string
}
```

### Result Types

```go
// No requeue needed
return reconcile.Result{}, nil

// Immediate requeue
return reconcile.Result{Requeue: true}, nil

// Requeue after delay
return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil

// Error triggers automatic retry
return reconcile.Result{}, fmt.Errorf("connection failed")
```

## BaseReconciler

Embed `BaseReconciler` for common functionality:

### Status Updates

```go
// Update resource status
device.Status.State = "running"
r.UpdateStatus(ctx, device)
```

### Event Emission

```go
// Emit event for state change
r.EmitEvent(ctx, "io.example.device.ready", device)
```

### Condition Management

```go
// Set Kubernetes-style condition
r.SetCondition(
    device,
    "Ready",           // type
    "True",            // status
    "DeviceRunning",   // reason
    "Device is ready", // message
)
```

## Work Queue

The controller uses a work queue for reconciliation requests:

### Features

- **Deduplication**: Same item only queued once
- **Rate Limiting**: Exponential backoff for failures
- **Graceful Shutdown**: Waits for in-flight requests
- **Thread-Safe**: Concurrent enqueue/dequeue

### Manual Enqueueing

```go
// Enqueue reconciliation request
request := reconcile.ReconcileRequest{
    ResourceKind: "Device",
    ResourceUID:  "dev-123",
    Reason:       "Manual trigger",
}
controller.Enqueue(request)

// Enqueue with delay
controller.EnqueueAfter(request, 30*time.Second)
```

### Rate Limiting

```go
// Create rate-limited queue
limiter := reconcile.NewExponentialBackoffRateLimiter(
    1*time.Second,   // base delay
    5*time.Minute,   // max delay
)
queue := reconcile.NewRateLimitedWorkQueue(limiter)

// Failed items are requeued with backoff:
// 1s, 2s, 4s, 8s, 16s, ... up to 5 minutes
```

## Event-Driven Reconciliation

The controller automatically reconciles resources when events occur:

```go
// Event published
event, _ := events.NewResourceEvent(
    "io.example.device.created",
    "Device",
    "dev-123",
    device,
)
eventBus.Publish(ctx, event)

// Controller automatically:
// 1. Detects event matches registered reconciler
// 2. Enqueues reconciliation request
// 3. Calls Reconcile() with loaded resource
```

### Event Subscription

The controller subscribes to event patterns:

```go
// Default: subscribe to all events
controller.Start(ctx)

// Custom: subscribe to specific pattern
// (modify controller.go if needed)
eventBus.Subscribe("io.example.device.**", handler)
```

## Advanced Patterns

### Periodic Reconciliation

Ensure eventual consistency with periodic checks:

```go
func (r *DeviceReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    device := resource.(*Device)

    // Do reconciliation work
    ...

    // Always requeue for periodic check
    return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}
```

### Dependency Management

Reconcile related resources:

```go
func (r *ClusterReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    cluster := resource.(*Cluster)

    // Reconcile dependent nodes
    for _, nodeUID := range cluster.Spec.Nodes {
        r.controller.Enqueue(reconcile.ReconcileRequest{
            ResourceKind: "Node",
            ResourceUID:  nodeUID,
            Reason:       "Cluster update",
        })
    }

    return reconcile.Result{}, nil
}
```

### Error Handling

```go
func (r *DeviceReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    device := resource.(*Device)

    // Transient error - retry with backoff
    if err := r.connect(device); err != nil {
        r.SetCondition(device, "Ready", "False", "ConnectError", err.Error())
        return reconcile.Result{}, err  // Auto-retry with exponential backoff
    }

    // Permanent error - don't retry
    if device.Spec.Config == nil {
        r.SetCondition(device, "Ready", "False", "InvalidConfig", "Config is nil")
        return reconcile.Result{}, nil  // No retry
    }

    // Temporary issue - retry after delay
    if !r.isReady(device) {
        return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
    }

    return reconcile.Result{}, nil
}
```

### Owner References

Track resource ownership:

```go
type Resource struct {
    Kind     string
    UID      string
    OwnerUID string  // Parent resource UID
}

func (r *NodeReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    node := resource.(*Node)

    // If owner deleted, delete this resource
    if node.OwnerUID != "" {
        owner, err := r.Client.Get(ctx, "Cluster", node.OwnerUID)
        if err != nil {
            // Owner deleted - delete this node
            return reconcile.Result{}, r.Client.Delete(ctx, "Node", node.UID)
        }
    }

    return reconcile.Result{}, nil
}
```

## Best Practices

1. **Be Idempotent**: Reconcile should work correctly when called multiple times
2. **Check Actual State**: Always verify current state before making changes
3. **Update Status**: Reflect actual state in Status, not Spec
4. **Emit Events**: Publish events for significant state changes
5. **Handle Errors**: Return errors for transient issues, nil for permanent ones
6. **Use Conditions**: Set conditions for human-readable status
7. **Periodic Checks**: Requeue periodically to ensure consistency
8. **Avoid Blocking**: Keep reconciliation fast, offload heavy work
9. **Log Appropriately**: Use logger for debugging, not fmt.Println
10. **Test Thoroughly**: Mock storage and events for unit tests

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/openchami/fabrica/pkg/events"
    "github.com/openchami/fabrica/pkg/reconcile"
    "github.com/openchami/fabrica/pkg/storage"
)

// Resource definition
type Device struct {
    Kind   string       `json:"kind"`
    UID    string       `json:"uid"`
    Spec   DeviceSpec   `json:"spec"`
    Status DeviceStatus `json:"status"`
}

type DeviceSpec struct {
    Desired string `json:"desired"`
    Config  string `json:"config"`
}

type DeviceStatus struct {
    State      string `json:"state"`
    LastSeen   string `json:"lastSeen"`
    Conditions []Condition `json:"conditions"`
}

type Condition struct {
    Type    string `json:"type"`
    Status  string `json:"status"`
    Reason  string `json:"reason"`
    Message string `json:"message"`
}

func (d *Device) GetKind() string { return d.Kind }
func (d *Device) GetUID() string  { return d.UID }

// Reconciler implementation
type DeviceReconciler struct {
    reconcile.BaseReconciler
}

func (r *DeviceReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    device := resource.(*Device)

    r.Logger.Infof("Reconciling device %s (desired: %s, current: %s)",
        device.UID, device.Spec.Desired, device.Status.State)

    // Handle desired state
    switch device.Spec.Desired {
    case "running":
        if device.Status.State != "running" {
            // Simulate starting device
            r.Logger.Infof("Starting device %s", device.UID)

            device.Status.State = "running"
            device.Status.LastSeen = time.Now().Format(time.RFC3339)

            r.SetCondition(device, "Ready", "True", "DeviceRunning", "Device is running")
            r.UpdateStatus(ctx, device)
            r.EmitEvent(ctx, "io.example.device.started", device)
        }

    case "stopped":
        if device.Status.State != "stopped" {
            // Simulate stopping device
            r.Logger.Infof("Stopping device %s", device.UID)

            device.Status.State = "stopped"

            r.SetCondition(device, "Ready", "False", "DeviceStopped", "Device is stopped")
            r.UpdateStatus(ctx, device)
            r.EmitEvent(ctx, "io.example.device.stopped", device)
        }
    }

    // Requeue after 5 minutes for periodic check
    return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *DeviceReconciler) GetResourceKind() string {
    return "Device"
}

func main() {
    // Set up infrastructure
    eventBus := events.NewInMemoryEventBus(1000, 10)
    eventBus.Start()
    defer eventBus.Close()

    storage := storage.NewInMemoryStorage()
    controller := reconcile.NewController(eventBus, storage)

    // Register reconciler
    reconciler := &DeviceReconciler{
        BaseReconciler: reconcile.BaseReconciler{
            EventBus: eventBus,
            Logger:   reconcile.NewDefaultLogger(),
        },
    }
    controller.RegisterReconciler(reconciler)

    // Start controller
    ctx := context.Background()
    controller.Start(ctx)
    defer controller.Stop()

    // Simulate creating a device
    device := &Device{
        Kind: "Device",
        UID:  "dev-123",
        Spec: DeviceSpec{
            Desired: "running",
            Config:  "default",
        },
        Status: DeviceStatus{
            State: "stopped",
        },
    }

    // Store device
    storage.Save(ctx, device.Kind, device.UID, device)

    // Publish event to trigger reconciliation
    event, _ := events.NewResourceEvent(
        "io.example.device.created",
        "Device",
        device.UID,
        device,
    )
    eventBus.Publish(ctx, *event)

    // Wait for reconciliation
    time.Sleep(2 * time.Second)

    // Output:
    // [INFO] Registered reconciler for Device
    // [INFO] Starting reconciliation controller with 5 workers
    // [INFO] Reconciling device dev-123 (desired: running, current: stopped)
    // [INFO] Starting device dev-123
    // [INFO] Emitted event io.example.device.started for Device/dev-123
}
```

## Next Steps

- [Events](events.md) - Integrate events with reconciliation
- [Storage](storage.md) - Persist resource state
- [Architecture](architecture.md) - System design patterns
