<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Events System

Fabrica provides a CloudEvents-compliant event system for building event-driven applications. The event system enables decoupled communication between components through a publish-subscribe pattern.

## Overview

The event system consists of:
- **Event Bus**: Central hub for publishing and subscribing to events
- **CloudEvents**: Standard event format for interoperability
- **Lifecycle Events**: Automatic events for resource CRUD operations (create, update, patch, delete)
- **Condition Events**: Events triggered when resource conditions change
- **Wildcard Subscriptions**: Pattern-based event routing
- **In-Memory Implementation**: Low-latency event delivery

## Event Types

Fabrica automatically publishes two categories of events:

### 1. Lifecycle Events
Published automatically when resources are created, updated, patched, or deleted:
- `{prefix}.{resource}.created` - Resource creation
- `{prefix}.{resource}.updated` - Resource updates via PUT
- `{prefix}.{resource}.patched` - Resource updates via PATCH
- `{prefix}.{resource}.deleted` - Resource deletion

### 2. Condition Events
Published automatically when resource conditions change:
- `{prefix}.condition.{conditiontype}` - When a condition status changes
- Includes full condition details and resource context
- Only published when condition status actually changes

## Quick Start

### Creating an Event Bus

```go
import "github.com/alexlovelltroy/fabrica/pkg/events"

// Create in-memory event bus
eventBus := events.NewInMemoryEventBus(
    1000,  // buffer size
    10,    // worker count
)

// Start processing events
eventBus.Start()
defer eventBus.Close()
```

### Publishing Events

```go
// Create a simple event
event, err := events.NewEvent(
    "io.example.user.created",  // event type
    "/users/user-123",           // source
    userData,                    // data payload
)
if err != nil {
    log.Fatal(err)
}

// Publish the event
err = eventBus.Publish(context.Background(), *event)
if err != nil {
    log.Fatal(err)
}
```

### Subscribing to Events

```go
// Subscribe to specific event type
id, err := eventBus.Subscribe(
    "io.example.user.created",
    func(ctx context.Context, event events.Event) error {
        fmt.Printf("User created: %s\n", event.ID())
        return nil
    },
)

// Unsubscribe when done
defer eventBus.Unsubscribe(id)
```

## Event Configuration

Configure Fabrica's automatic event publishing with `EventConfig`:

### Basic Configuration

```go
import "github.com/alexlovelltroy/fabrica/pkg/events"

// Configure event system
config := &events.EventConfig{
    Enabled:                true,  // Enable/disable all events
    LifecycleEventsEnabled: true,  // Enable CRUD operation events
    ConditionEventsEnabled: true,  // Enable condition change events
    EventTypePrefix:        "io.fabrica",           // Event type prefix
    ConditionEventPrefix:   "io.fabrica.condition", // Condition event prefix
    Source:                 "inventory-api",        // Event source identifier
}

// Apply configuration globally
events.SetEventConfig(config)
```

### Environment Variables

Configure events via environment variables in generated servers:

```bash
# Enable/disable event publishing
FABRICA_EVENTS_ENABLED=true
FABRICA_LIFECYCLE_EVENTS_ENABLED=true
FABRICA_CONDITION_EVENTS_ENABLED=true

# Customize event prefixes
FABRICA_EVENT_PREFIX=io.mycompany.inventory
FABRICA_CONDITION_EVENT_PREFIX=io.mycompany.inventory.condition
FABRICA_EVENT_SOURCE=production-api
```

### Generated Event Types

With prefix `io.fabrica` and resource `Device`:

**Lifecycle Events:**
- `io.fabrica.device.created` - Device creation
- `io.fabrica.device.updated` - Device update (PUT)
- `io.fabrica.device.patched` - Device patch (PATCH)
- `io.fabrica.device.deleted` - Device deletion

**Condition Events:**
- `io.fabrica.condition.ready` - Ready condition changed
- `io.fabrica.condition.healthy` - Healthy condition changed
- `io.fabrica.condition.available` - Available condition changed

## CloudEvents

Fabrica uses the [CloudEvents](https://cloudevents.io/) specification for all events.

### Event Structure

Every event contains:
- **ID**: Unique identifier (auto-generated)
- **Type**: Event type (e.g., "io.example.user.created")
- **Source**: Event source URI (e.g., "/users/user-123")
- **Time**: Timestamp (auto-set)
- **Data**: Event payload (JSON)
- **Extensions**: Custom attributes

### Creating Resource Events

For resource-specific events, use `NewResourceEvent`:

```go
event, err := events.NewResourceEvent(
    "io.example.device.connected",  // event type
    "Device",                        // resource kind
    "dev-abc123",                    // resource UID
    deviceData,                      // data payload
)
```

This automatically:
- Sets the source to `/resources/{kind}/{uid}`
- Adds `resourcekind` extension attribute
- Adds `resourceuid` extension attribute

### Accessing Event Data

```go
func handleEvent(ctx context.Context, event events.Event) error {
    // Basic attributes
    fmt.Printf("ID: %s\n", event.ID())
    fmt.Printf("Type: %s\n", event.Type())
    fmt.Printf("Source: %s\n", event.Source())
    fmt.Printf("Time: %s\n", event.Time())

    // Resource extensions (if present)
    kind := event.ResourceKind()
    uid := event.ResourceUID()

    // Event data
    var data MyDataType
    err := event.DataAs(&data)
    if err != nil {
        return err
    }

    return nil
}
```

## Wildcard Subscriptions

Subscribe to multiple event types using wildcards:

### Single Wildcard (`*`)

Matches exactly one segment:

```go
// Matches: io.example.user.created, io.example.user.updated
// Does NOT match: io.example.user.group.created
eventBus.Subscribe("io.example.user.*", handler)
```

### Multi Wildcard (`**`)

Matches one or more segments:

```go
// Matches: io.example.user.created
//          io.example.user.group.created
//          io.example.user.x.y.z
eventBus.Subscribe("io.example.user.**", handler)
```

### Pattern Examples

```go
// All events
eventBus.Subscribe("**", handler)

// All events for a specific resource kind
eventBus.Subscribe("io.example.device.**", handler)

// Specific operation across all resources
eventBus.Subscribe("io.example.*.created", handler)

// Exact match
eventBus.Subscribe("io.example.device.connected", handler)
```

## Event Types

### Naming Convention

Use reverse domain notation:
```
{domain}.{application}.{resource}.{action}
```

Examples:
- `io.example.user.created`
- `io.example.device.connected`
- `io.example.order.shipped`
- `io.example.payment.completed`

### Common Event Types

**Create/Update/Delete:**
```go
events.NewResourceEvent("io.example.device.created", kind, uid, resource)
events.NewResourceEvent("io.example.device.updated", kind, uid, resource)
events.NewResourceEvent("io.example.device.deleted", kind, uid, resource)
```

**State Changes:**
```go
events.NewResourceEvent("io.example.device.connected", kind, uid, resource)
events.NewResourceEvent("io.example.device.disconnected", kind, uid, resource)
events.NewResourceEvent("io.example.device.failed", kind, uid, resource)
```

**Operations:**
```go
events.NewResourceEvent("io.example.order.shipped", kind, uid, order)
events.NewResourceEvent("io.example.payment.processed", kind, uid, payment)
```

## Condition Events

Fabrica automatically publishes events when resource conditions change, following Kubernetes condition patterns.

### What are Conditions?

Conditions represent the current state of a resource:

```go
type Condition struct {
    Type               string    `json:"type"`               // "Ready", "Healthy", "Available"
    Status             string    `json:"status"`             // "True", "False", "Unknown"
    LastTransitionTime time.Time `json:"lastTransitionTime"` // When status last changed
    Reason             string    `json:"reason,omitempty"`   // Machine-readable reason
    Message            string    `json:"message,omitempty"`  // Human-readable message
}
```

### Automatic Condition Events

When you update resource conditions, events are published automatically:

```go
import "github.com/alexlovelltroy/fabrica/pkg/resource"

// This will publish a condition event if the status changes
changed := resource.SetResourceCondition(ctx, device,
    "Ready",           // condition type
    "True",            // status
    "DeviceOnline",    // reason
    "Device is operational" // message
)

if changed {
    // Event published: "io.fabrica.condition.ready"
    log.Println("Ready condition changed - event published")
}
```

### Condition Event Format

Condition events use the CloudEvents format:

```json
{
  "specversion": "1.0",
  "type": "io.fabrica.condition.ready",
  "source": "inventory-api",
  "id": "condition-event-abc123",
  "time": "2025-10-21T15:30:45Z",
  "datacontenttype": "application/json",
  "subject": "devices/dev-123",
  "data": {
    "resourceKind": "Device",
    "resourceUID": "dev-123",
    "condition": {
      "type": "Ready",
      "status": "True",
      "reason": "DeviceOnline",
      "message": "Device is operational",
      "lastTransitionTime": "2025-10-21T15:30:45Z"
    }
  }
}
```

### Working with Condition Events

Subscribe to condition events using wildcards:

```go
// All condition events
eventBus.Subscribe("io.fabrica.condition.**", handleConditionEvent)

// Specific condition type
eventBus.Subscribe("io.fabrica.condition.ready", handleReadyCondition)

// Condition events for specific resource
eventBus.Subscribe("io.fabrica.condition.*", func(ctx context.Context, event events.Event) error {
    var conditionData struct {
        ResourceKind string `json:"resourceKind"`
        ResourceUID  string `json:"resourceUID"`
        Condition    struct {
            Type   string `json:"type"`
            Status string `json:"status"`
            Reason string `json:"reason"`
        } `json:"condition"`
    }

    if err := event.DataAs(&conditionData); err != nil {
        return err
    }

    if conditionData.ResourceKind == "Device" && conditionData.Condition.Type == "Ready" {
        if conditionData.Condition.Status == "False" {
            // Send alert - device became not ready
            sendDeviceAlert(conditionData.ResourceUID, conditionData.Condition.Reason)
        }
    }

    return nil
})
```

### Common Condition Types

**Standard Kubernetes-style conditions:**
- `Ready` - Resource is ready to serve requests
- `Available` - Resource is available for use
- `Progressing` - Resource is making progress toward desired state

**Custom application conditions:**
- `Healthy` - Health check status
- `Connected` - Network connectivity status
- `Authenticated` - Authentication status
- `Validated` - Data validation status

### Lifecycle vs Condition Events

| Aspect | Lifecycle Events | Condition Events |
|--------|-----------------|------------------|
| **Trigger** | CRUD operations | Condition status changes |
| **Frequency** | Every operation | Only when status changes |
| **Data** | Full resource | Condition + resource context |
| **Use Cases** | Audit, integration | Monitoring, alerting |
| **Examples** | `device.created`, `user.updated` | `condition.ready`, `condition.healthy` |

## In-Memory Event Bus

The in-memory implementation provides:
- **Low Latency**: Microsecond event delivery
- **Thread-Safe**: Concurrent publish/subscribe
- **Non-Blocking**: Asynchronous event handling
- **Buffered Queue**: Configurable event buffer

### Configuration

```go
eventBus := events.NewInMemoryEventBus(
    bufferSize,   // Event queue buffer size (default: 1000)
    workerCount,  // Number of worker goroutines (default: 10)
)
```

**Buffer Size:**
- Larger buffer = more events queued before blocking
- Smaller buffer = less memory, faster backpressure

**Worker Count:**
- More workers = higher throughput
- Fewer workers = lower resource usage

### Characteristics

**Advantages:**
- Very fast (in-process)
- No external dependencies
- Perfect for single-instance apps
- Great for development/testing

**Limitations:**
- No persistence (events lost on restart)
- No cross-instance delivery
- Limited to single process

## Advanced Usage

### Error Handling

Event handlers should return errors:

```go
handler := func(ctx context.Context, event events.Event) error {
    if err := processEvent(event); err != nil {
        // Error is logged but doesn't stop processing
        return fmt.Errorf("failed to process event: %w", err)
    }
    return nil
}
```

### Context Propagation

Use context for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := eventBus.Publish(ctx, event)
if err != nil {
    // Handle timeout or cancellation
}
```

### Custom Extensions

Add custom attributes to events:

```go
event.SetExtension("tenant", "acme-corp")
event.SetExtension("priority", "high")
event.SetExtension("region", "us-west")

// Access in handler
tenant := event.Extensions()["tenant"]
```

### Multiple Subscriptions

One event can trigger multiple handlers:

```go
// Logging handler
eventBus.Subscribe("**", loggingHandler)

// Metrics handler
eventBus.Subscribe("**", metricsHandler)

// Business logic handler
eventBus.Subscribe("io.example.order.*", orderHandler)
```

## Integration Patterns

### Event Sourcing

```go
type EventStore struct {
    bus events.EventBus
}

func (s *EventStore) SaveAggregate(aggregate Aggregate) error {
    // Save events
    for _, event := range aggregate.Events() {
        if err := s.bus.Publish(ctx, event); err != nil {
            return err
        }
    }
    return nil
}
```

### CQRS (Command Query Responsibility Segregation)

```go
// Command side publishes events
func CreateUser(cmd CreateUserCommand) error {
    user := newUser(cmd)
    event, _ := events.NewResourceEvent(
        "io.example.user.created",
        "User",
        user.UID,
        user,
    )
    return eventBus.Publish(ctx, event)
}

// Query side subscribes to events
eventBus.Subscribe("io.example.user.**", func(ctx context.Context, e events.Event) error {
    // Update read model
    return updateReadModel(e)
})
```

### Saga Pattern

```go
// Order saga
eventBus.Subscribe("io.example.order.created", func(ctx context.Context, e events.Event) error {
    // Reserve inventory
    // Process payment
    // Ship order
    return nil
})
```

## Best Practices

1. **Use Structured Event Types**: Follow reverse domain notation
2. **Include Context**: Add relevant extensions for filtering
3. **Keep Handlers Fast**: Offload heavy work to background jobs
4. **Handle Errors**: Return errors from handlers for logging
5. **Version Your Events**: Plan for schema evolution
6. **Test Event Flow**: Use in-memory bus for integration tests
7. **Monitor Events**: Subscribe to `**` for metrics/logging

## Example: Complete Event-Driven System

```go
package main

import (
    "context"
    "fmt"
    "github.com/alexlovelltroy/fabrica/pkg/events"
)

type Device struct {
    UID    string
    Name   string
    Status string
}

func main() {
    // Create event bus
    bus := events.NewInMemoryEventBus(1000, 10)
    bus.Start()
    defer bus.Close()

    // Subscribe to all device events
    bus.Subscribe("io.example.device.**", func(ctx context.Context, e events.Event) error {
        fmt.Printf("[LOG] Device event: %s\n", e.Type())
        return nil
    })

    // Subscribe to connected events
    bus.Subscribe("io.example.device.connected", func(ctx context.Context, e events.Event) error {
        var device Device
        e.DataAs(&device)
        fmt.Printf("[NOTIFY] Device %s is now online\n", device.Name)
        return nil
    })

    // Publish event
    device := Device{
        UID:    "dev-123",
        Name:   "Sensor-01",
        Status: "connected",
    }

    event, _ := events.NewResourceEvent(
        "io.example.device.connected",
        "Device",
        device.UID,
        device,
    )

    bus.Publish(context.Background(), *event)

    // Output:
    // [LOG] Device event: io.example.device.connected
    // [NOTIFY] Device Sensor-01 is now online
}
```

## Next Steps

- [Reconciliation](reconciliation.md) - Use events to trigger reconciliation
- [Architecture](architecture.md) - Event system design
- [Examples](examples.md) - Real-world event patterns
