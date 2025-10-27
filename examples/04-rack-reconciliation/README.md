<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 4: Rack Reconciliation with Event-Driven Architecture

**Time to complete:** ~45 minutes
**Difficulty:** Advanced
**Prerequisites:** Go 1.23+, fabrica CLI installed, understanding of reconciliation patterns

## What You'll Build

A data center rack inventory system that demonstrates **event-driven reconciliation**:
- **RackTemplate** - Defines rack configurations (chassis, blades, nodes, BMCs)
- **Rack** - References a template and automatically provisions child resources
- **Reconciliation Controller** - Watches for Rack creation events and provisions infrastructure
- **Hierarchical Resources** - Chassis → Blades → Nodes/BMCs with proper relationships

This example demonstrates how Fabrica's reconciliation framework enables Kubernetes-style declarative resource management.

## Architecture Overview

```
User Creates Rack
       ↓
Event Published (io.fabrica.rack.created)
       ↓
Reconciliation Controller Receives Event
       ↓
RackReconciler.Reconcile() Called
       ↓
Load RackTemplate
       ↓
Create Child Resources:
  - 2 Chassis
  - 8 Blades (4 per chassis)
  - 16 Nodes (2 per blade)
  - 8 BMCs (shared mode: 1 per blade)
       ↓
Update Rack Status (Phase: Ready)
```

## Reconciliation Pattern

The reconciliation pattern is a core concept in Kubernetes and declarative systems:

1. **Desired State** - User declares what they want (Rack with template)
2. **Current State** - System tracks what exists (child resources)
3. **Reconciliation Loop** - Controller continuously works to make current state match desired state
4. **Event-Driven** - Changes trigger reconciliation automatically

**Key Benefits:**
- Self-healing: If resources are deleted, reconciliation recreates them
- Declarative: Users specify "what" not "how"
- Asynchronous: Long-running operations don't block API calls
- Scalable: Multiple reconcilers can run concurrently

## Step-by-Step Guide

### Step 1: Initialize Project with Events and Reconciliation

```bash
# Create project with reconciliation enabled
fabrica init rack-inventory \
  --module github.com/example/rack-inventory \
  --storage-type file \
  --events \
  --events-bus memory \
  --reconcile

cd rack-inventory
```

**What gets created:**
```
rack-inventory/
├── .fabrica.yaml                    # Configuration with reconciliation enabled
├── cmd/
│   └── server/
│       └── main.go                  # Server with controller setup (commented)
├── pkg/
│   └── resources/                   # Empty (for resources)
└── internal/
    └── storage/
```

The `.fabrica.yaml` file will include:
```yaml
features:
  events:
    enabled: true
    bus_type: memory
  reconciliation:
    enabled: true
    worker_count: 5
    requeue_delay: 5
generation:
  reconciliation: true
```

### Step 2: Add All Required Resources

The rack reconciliation example requires six resources:

```bash
# Template resource
fabrica add resource RackTemplate

# Parent rack resource
fabrica add resource Rack

# Child resources (created by reconciler)
fabrica add resource Chassis
fabrica add resource Blade
fabrica add resource BMC
fabrica add resource Node
```

### Step 3: Define Resource Structures

Copy the resource definitions from this example directory to your project:

```bash
# From the fabrica repository
cp -r ../examples/04-rack-reconciliation/pkg/resources/* pkg/resources/
```

Or create each resource file manually following the structures below.

#### RackTemplate Resource

[pkg/resources/racktemplate/racktemplate.go](pkg/resources/racktemplate/racktemplate.go)

Defines the configuration template for a rack:
- `ChassisCount` - Number of chassis in the rack
- `ChassisConfig.BladeCount` - Number of blades per chassis
- `ChassisConfig.BladeConfig.NodeCount` - Number of nodes per blade
- `ChassisConfig.BladeConfig.BMCMode` - "shared" or "dedicated" BMC per blade/node

#### Rack Resource

[pkg/resources/rack/rack.go](pkg/resources/rack/rack.go)

Represents a physical rack that references a RackTemplate:
- `Spec.TemplateUID` - References which template to use
- `Spec.Location` - Physical location information
- `Status.Phase` - "Pending", "Provisioning", "Ready", "Error"
- `Status.ChassisUIDs` - UIDs of created chassis
- `Status.Total*` - Counts of provisioned resources

#### Child Resources

- **Chassis** [pkg/resources/chassis/chassis.go](pkg/resources/chassis/chassis.go) - Contains blades
- **Blade** [pkg/resources/blade/blade.go](pkg/resources/blade/blade.go) - Contains nodes and BMCs
- **BMC** [pkg/resources/bmc/bmc.go](pkg/resources/bmc/bmc.go) - Management controller
- **Node** [pkg/resources/node/node.go](pkg/resources/node/node.go) - Compute node

### Step 4: Generate All Code (Including Reconcilers!)

```bash
fabrica generate
```

### Step 5: Update Dependencies

After generation, update your Go module dependencies:

```bash
go mod tidy
```

This resolves all the new imports that were added by the code generator.

**What `fabrica generate` creates:**
- HTTP handlers for all 6 resources
- Storage layer
- Request/response models
- Route registration
- OpenAPI spec
- **Reconcilers for all 6 resources** (because reconciliation is enabled in config)
- **Reconciler registration boilerplate**

**After running `fabrica generate`, you'll have:**
```
pkg/reconcilers/
├── rack_reconciler_generated.go         # Generated boilerplate (DO NOT EDIT)
├── rack_reconciler.go                   # User implementation (SAFE TO EDIT)
├── racktemplate_reconciler_generated.go # Generated boilerplate
├── racktemplate_reconciler.go           # User stub (edit to add logic)
├── chassis_reconciler_generated.go      # Generated boilerplate
├── chassis_reconciler.go                # User stub (edit to add logic)
├── blade_reconciler_generated.go        # Generated boilerplate
├── blade_reconciler.go                  # User stub (edit to add logic)
├── bmc_reconciler_generated.go          # Generated boilerplate
├── bmc_reconciler.go                    # User stub (edit to add logic)
├── node_reconciler_generated.go         # Generated boilerplate
├── node_reconciler.go                   # User stub (edit to add logic)
├── registration_generated.go            # Generated registration
└── event_handlers_generated.go          # Generated event handlers
```

**File Structure:**
- `*_reconciler_generated.go` - Generated boilerplate, overwritten on each `fabrica generate`
- `*_reconciler.go` - User-editable implementation, created once and never overwritten
- `registration_generated.go` - Generated registration code, overwritten on each generate
- `event_handlers_generated.go` - Generated event handlers, overwritten on each generate

### Step 5: Implement Custom Reconciliation Logic

The generated reconcilers create two files for each resource:

1. **`{resource}_reconciler_generated.go`** - Generated boilerplate (DO NOT EDIT)
   - Struct definition
   - Factory function
   - Orchestration wrapper with status updates, events, error handling
   - Requeue logic

2. **`{resource}_reconciler.go`** - User-editable stub (SAFE TO EDIT)
   - `reconcile{Resource}()` method with TODO comment
   - This is where you implement custom business logic

For the Rack resource, edit `pkg/reconcilers/rack_reconciler.go`:

```go
// This method is in rack_reconciler.go - safe to edit
func (r *RackReconciler) reconcileRack(ctx context.Context, res *rack.Rack) error {
    // 1. Check if already reconciled (idempotency)
    if res.Status.Phase == "Ready" {
        return nil
    }

    // 2. Load dependencies (RackTemplate)
    template := r.loadTemplate(ctx, res.Spec.TemplateUID)

    // 3. Create child resources
    for i := 0; i < template.Spec.ChassisCount; i++ {
        chassis := r.createChassis(ctx, res, i, template)
        // ... create blades, nodes, BMCs
    }

    // 4. Update parent status
    res.Status.Phase = "Ready"
    res.Status.TotalChassis = template.Spec.ChassisCount
    // Status will be saved automatically by the generated wrapper

    return nil
}
```

**Key points:**
- The `_generated.go` file handles all orchestration - you just implement the logic
- Your custom code in `_reconciler.go` is NEVER overwritten on regeneration
- Running `fabrica generate` again is safe - it only updates `_generated.go` files

**You can copy the example implementation:**
```bash
# Copy the complete rack reconciler implementation
cp ../examples/04-rack-reconciliation/pkg/reconcilers/rack_reconciler.go \
   pkg/reconcilers/rack_reconciler.go
```

This example file shows a complete production-ready implementation with:
- Template-based resource provisioning
- Hierarchical resource creation (Rack → Chassis → Blades → Nodes/BMCs)
- Idempotent reconciliation logic
- Proper error handling and status updates
- Helper methods for creating child resources

### Step 6: Uncomment Generated Code in main.go

The `fabrica init --reconcile` command created `cmd/server/main.go` with reconciliation controller setup already in place (but commented out).

Edit `cmd/server/main.go` and uncomment these lines:

```go
// Around line 280-310, uncomment:
storageBackend, _ := storage.GetBackend()
controller = reconcile.NewController(eventBus, storageBackend)

if err := reconcilers.RegisterReconcilers(controller); err != nil {
    log.Fatalf("Failed to register reconcilers: %v", err)
}

if err := controller.Start(ctx); err != nil {
    log.Fatalf("Failed to start reconciliation controller: %v", err)
}
defer controller.Stop()
```

### Step 7: Build and Run

```bash
# Build
go build -o rack-server ./cmd/server

# Run
./rack-server
```

Expected output:
```
2025/10/18 12:00:00 Reconciliation controller started
2025/10/18 12:00:00 Registered reconciler for resource kind: Rack
2025/10/18 12:00:00 Controller started with 3 workers
2025/10/18 12:00:00 Server starting on :8080
```

## Testing Rack Reconciliation

### 1. Create a RackTemplate

```bash
# Create template defining a standard rack configuration
curl -X POST http://localhost:8080/racktemplates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "standard-rack",
    "chassisCount": 2,
    "chassisConfig": {
    "bladeCount": 4,
    "bladeConfig": {
      "nodeCount": 2,
      "bmcMode": "shared"
      }
    },
      "description": "Standard 2-chassis rack with 4 blades per chassis"
    }
  }'
```

Save the UID from the response (e.g., `rktmpl-abc123`).

### 2. Create a Rack (Triggers Reconciliation)

```bash
# Create rack referencing the template
curl -X POST http://localhost:8080/racks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "rack-01",
    "templateUID": "rktmpl-abc123",
    "location": "datacenter-1",
    "datacenter": "DC1",
    "row": "A",
    "position": "01"
    }
  }'
```

**What Happens:**
1. API handler creates the Rack resource with `Status.Phase = "Pending"`
2. Publishes `io.fabrica.rack.created` event to event bus
3. Reconciliation controller receives event
4. Enqueues reconciliation request for the Rack
5. Worker picks up request and calls `RackReconciler.Reconcile()`
6. Reconciler:
   - Loads RackTemplate
   - Creates 2 Chassis resources
   - Creates 8 Blade resources (4 per chassis)
   - Creates 8 BMC resources (shared mode: 1 per blade)
   - Creates 16 Node resources (2 per blade)
   - Updates Rack status to `Phase = "Ready"` with counts

### 3. Check Reconciliation Results

```bash
# Get the rack status (should show Phase: Ready)
curl http://localhost:8080/racks/rack-abc123

# Response shows:
{
  "apiVersion": "v1",
  "kind": "Rack",
  "metadata": {
    "name": "rack-01",
    "uid": "rack-abc123",
    ...
  },
  "spec": {
    "templateUID": "rktmpl-abc123",
    "location": "datacenter-1",
    ...
  },
  "status": {
    "phase": "Ready",
    "chassisUIDs": ["chas-xyz1", "chas-xyz2"],
    "totalChassis": 2,
    "totalBlades": 8,
    "totalNodes": 16,
    "totalBMCs": 8
  }
}
```

### 4. Verify Child Resources Were Created

```bash
# List all chassis
curl http://localhost:8080/chassis
# Returns 2 chassis

# List all blades
curl http://localhost:8080/blades
# Returns 8 blades

# List all nodes
curl http://localhost:8080/nodes
# Returns 16 nodes

# List all BMCs
curl http://localhost:8080/bmcs
# Returns 8 BMCs (shared mode)

# Get specific chassis to see its blades
curl http://localhost:8080/chassis/chas-xyz1
# Shows bladeUIDs: [...] in status
```

### 5. Test Different BMC Modes

```bash
# Create template with dedicated BMC mode (1 BMC per node)
curl -X POST http://localhost:8080/racktemplates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dedicated-bmc-rack",
    "chassisCount": 1,
    "chassisConfig": {
      "bladeCount": 2,
      "bladeConfig": {
        "nodeCount": 4,
        "bmcMode": "dedicated"
      }
    }
  }'

# Create rack with this template
curl -X POST http://localhost:8080/racks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "rack-02",
    "templateUID": "<template-uid>",
    "location": "datacenter-1"
  }'

# Wait a few seconds, then check
curl http://localhost:8080/racks/<rack-uid>

# Should show:
# - 1 chassis
# - 2 blades
# - 8 nodes (4 per blade)
# - 8 BMCs (dedicated mode: 1 per node)
```

## Understanding the Event Flow

### Event-Driven Reconciliation Flow

```
1. HTTP POST /racks
   ↓
2. CreateRack Handler
   - Validates request
   - Creates Rack resource (Phase: Pending)
   - Saves to storage
   - Publishes event: events.NewResourceEvent("io.fabrica.rack.created", "Rack", uid, rack)
   ↓
3. Event Bus
   - Delivers event to all subscribers
   ↓
4. Reconciliation Controller
   - Receives event
   - Filters by resource kind ("Rack")
   - Enqueues reconcile request: {kind: "Rack", uid: "rack-abc123"}
   ↓
5. Worker Pool
   - Worker goroutine dequeues request
   - Loads resource from storage
   - Calls RackReconciler.Reconcile(ctx, rack)
   ↓
6. RackReconciler
   - Loads RackTemplate
   - Creates child resources (Chassis, Blades, Nodes, BMCs)
   - Updates Rack status (Phase: Ready)
   - Returns Result{RequeueAfter: 10m}
   ↓
7. Controller (if RequeueAfter > 0)
   - Schedules next reconciliation in 10 minutes
   - Allows periodic status checks
```

### Event Types

The system publishes events for resource lifecycle:

```go
// Created events trigger initial reconciliation
io.fabrica.rack.created
io.fabrica.racktemplate.created

// Updated events can trigger re-reconciliation
io.fabrica.rack.updated

// Custom events from reconcilers
io.fabrica.rack.provisioned
```

### Reconciliation Results

The reconciler returns a `reconcile.Result` to control re-queuing:

```go
// Success, don't requeue
return reconcile.Result{}, nil

// Success, requeue after 5 minutes (periodic check)
return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil

// Error, will be retried with exponential backoff
return reconcile.Result{}, fmt.Errorf("failed to create chassis: %w", err)

// Requeue immediately (use sparingly)
return reconcile.Result{Requeue: true}, nil
```

## Reconciler Best Practices

### 1. Idempotency

Reconcilers must be idempotent - safe to call multiple times:

```go
func (r *RackReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    rack := resource.(*rack.Rack)

    // Check if already reconciled
    if rack.Status.Phase == "Ready" {
        // Nothing to do, just requeue for periodic check
        return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
    }

    // Do work...
}
```

### 2. Error Handling

Handle errors gracefully and update status:

```go
// Load dependency
template, err := r.loadTemplate(ctx, rack.Spec.TemplateUID)
if err != nil {
    // Update status to indicate error
    rack.Status.Phase = "Error"
    rack.Status.Message = fmt.Sprintf("Failed to load template: %v", err)
    r.Storage.Save(ctx, rack.Kind, rack.UID, rack)

    // Return error for retry
    return reconcile.Result{}, err
}
```

### 3. Progressive Status Updates

Update status as work progresses:

```go
// Start
rack.Status.Phase = "Provisioning"
r.Storage.Save(ctx, rack.Kind, rack.UID, rack)

// Do work...

// Complete
rack.Status.Phase = "Ready"
rack.Status.TotalChassis = len(chassisUIDs)
r.Storage.Save(ctx, rack.Kind, rack.UID, rack)
```

### 4. Ownership and Relationships

Track ownership for garbage collection:

```go
// Child resources reference parent
chassis.Spec.RackUID = rack.GetUID()

// Parent tracks children
rack.Status.ChassisUIDs = append(rack.Status.ChassisUIDs, chassis.GetUID())
```

### 5. Use Conditions for Detailed Status

```go
import "github.com/openchami/fabrica/pkg/resource"

// Set conditions during reconciliation
resource.SetCondition(&rack.Status.Conditions, "Ready", "False", "Provisioning", "Creating chassis")
resource.SetCondition(&rack.Status.Conditions, "Ready", "True", "Provisioned", "All resources created")
```

## Advanced Topics

### Multiple Reconcilers

Register multiple reconcilers for different resources:

```go
controller := reconcile.NewController(eventBus, storage)

// Each resource can have its own reconciler
controller.RegisterReconciler(rackReconciler)
controller.RegisterReconciler(chassisReconciler)
controller.RegisterReconciler(nodeReconciler)

controller.Start(ctx)
```

### Reconciliation Chains

Child resources can trigger their own reconciliation:

```
Rack Created
  ↓ (reconciler creates)
Chassis Created
  ↓ (publishes event)
Chassis Reconciler
  ↓ (provisions hardware)
Chassis Ready
```

### Deletion and Cleanup

Handle resource deletion:

```go
func (r *RackReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    rack := resource.(*rack.Rack)

    // Check for deletion timestamp
    if !rack.Metadata.DeletionTimestamp.IsZero() {
        // Clean up child resources
        for _, chassisUID := range rack.Status.ChassisUIDs {
            r.Storage.Delete(ctx, "Chassis", chassisUID)
        }
        // Remove finalizer
        return reconcile.Result{}, nil
    }

    // Normal reconciliation...
}
```

### Watching External Systems

Reconcilers can integrate with external systems:

```go
func (r *RackReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    rack := resource.(*rack.Rack)

    // Call external API (e.g., IPAM, DNS, monitoring)
    ipAddress, err := r.IPAMClient.AllocateIP(rack.Spec.Location)
    if err != nil {
        return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
    }

    rack.Status.IPAddress = ipAddress
    // ...
}
```

## Troubleshooting

### Issue: Reconciliation Never Runs

**Symptoms:** Rack stays in "Pending" phase

**Causes:**
1. Controller not started
2. Reconciler not registered
3. Event not published

**Fixes:**
```bash
# Check logs for controller startup
grep "Controller started" server.log

# Check reconciler registration
grep "Registered reconciler" server.log

# Verify event bus is running
grep "Event bus started" server.log
```

### Issue: Reconciliation Fails with Errors

**Symptoms:** Rack status shows "Error" phase

**Debug:**
```bash
# Check reconciler logs
grep "Reconciling Rack" server.log
grep "Failed to" server.log

# Check resource status
curl http://localhost:8080/racks/<uid> | jq '.status'
```

### Issue: Child Resources Not Created

**Symptoms:** Rack shows Ready but no chassis exist

**Check:**
```bash
# Verify template exists
curl http://localhost:8080/racktemplates/<template-uid>

# Check reconciler can access storage
ls -la ./data/
```

### Issue: Reconciliation Runs Too Often

**Symptoms:** High CPU usage, excessive log messages

**Fix:** Increase `RequeueAfter` duration:
```go
// Instead of frequent checks
return reconcile.Result{RequeueAfter: 30 * time.Second}, nil

// Use longer intervals for stable resources
return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
```

## Comparison with Tests

This example mirrors the integration tests in [`test/integration/rack_reconciliation_functional_test.go`](../../test/integration/rack_reconciliation_functional_test.go):

| Test | Example |
|------|---------|
| Creates resources programmatically | Creates via HTTP API |
| Uses in-memory storage | Uses file-based storage |
| Synchronous validation | Asynchronous via HTTP |
| Test assertions | Manual verification with curl |
| Embedded in test suite | Standalone runnable project |

Both demonstrate the same reconciliation behavior:
- RackTemplate defines configuration
- Rack references template
- Reconciler provisions child resources
- Status tracks progress and results

## Next Steps

- **Add Deletion Reconciliation** - Clean up child resources when Rack is deleted
- **Add Update Reconciliation** - Handle template changes
- **Add Status Conditions** - Track detailed provisioning progress
- **Add Finalizers** - Prevent deletion until cleanup completes
- **Add Webhooks** - Validate resources before admission
- **Add Metrics** - Track reconciliation latency and errors
- **Multiple Reconcilers** - Add reconcilers for Chassis, Blade, etc.

## Production Considerations

### Scaling Reconcilers

```go
// Increase worker count for high throughput
controller := reconcile.NewController(eventBus, storage)
controller.SetWorkerCount(10) // Default is 3
```

### Persistent Event Bus

For production, use NATS or Kafka instead of in-memory:

```bash
fabrica init rack-inventory \
  --events \
  --events-bus nats \
  --nats-url nats://localhost:4222
```

### Database Storage

Switch to database storage for production:

```bash
fabrica init rack-inventory \
  --storage-type ent \
  --db postgres
```

### Monitoring

Add metrics to track reconciliation:

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    reconcileLatency = prometheus.NewHistogramVec(...)
    reconcileErrors = prometheus.NewCounterVec(...)
)

func (r *RackReconciler) Reconcile(...) {
    start := time.Now()
    defer func() {
        reconcileLatency.WithLabelValues("Rack").Observe(time.Since(start).Seconds())
    }()
    // ...
}
```

## Summary

This example demonstrates:
- ✅ **Event-Driven Architecture** - Resources trigger reconciliation via events
- ✅ **Reconciliation Pattern** - Controllers continuously reconcile desired vs actual state
- ✅ **Declarative API** - Users declare what they want, system figures out how
- ✅ **Hierarchical Resources** - Parent resources create and manage children
- ✅ **Status Management** - Track provisioning progress and results
- ✅ **Asynchronous Operations** - Long-running work doesn't block API
- ✅ **Production Patterns** - Error handling, idempotency, status updates
- ✅ **Code Generation** - Fabrica generates 70% of reconciler boilerplate
- ✅ **Simple Workflow** - Just `--reconcile` flag enables full reconciliation framework

**What Fabrica Generates for You:**
- Reconciler scaffolding for each resource
- Event subscription and routing
- Status update coordination
- Condition management
- Requeue logic with exponential backoff
- Worker pool management
- Registration boilerplate

**What You Implement:**
- Custom `reconcile{Resource}()` business logic (the "what to do")
- Resource-specific state observation
- Decision logic for reconciliation actions

All the infrastructure is generated - you just add the business logic!
