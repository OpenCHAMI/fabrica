<!--
Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Rack Reconciliation Integration Test

This directory contains test resources for demonstrating Fabrica's reconciliation and state machine capabilities with a hierarchical resource structure.

## Overview

The rack reconciliation test demonstrates:

1. **Hierarchical Resource Management**: Creating complex resource hierarchies with parent-child relationships
2. **Template-Based Provisioning**: Using RackTemplate to define and instantiate infrastructure
3. **Reconciliation Workflows**: Automatic creation of child resources when parent resources are created
4. **State Management**: Tracking provisioning state through resource lifecycle

## Resource Hierarchy

```
RackTemplate (defines structure)
    ↓
Rack (references template, triggers reconciliation)
    ↓
Chassis (2 per rack)
    ↓
Blade (4 per chassis = 8 total)
    ↓
├── BMC (1 per blade in "shared" mode, or 1 per node in "dedicated" mode)
└── Node (2 per blade = 16 total)
```

## Resources

### RackTemplate
Defines the structure and configuration for a rack:
- **ChassisCount**: Number of chassis in the rack
- **ChassisConfig**: Configuration for each chassis
  - **BladeCount**: Number of blades per chassis
  - **BladeConfig**: Configuration for each blade
    - **NodeCount**: Number of nodes per blade (1-8)
    - **BMCMode**: "shared" (1 BMC per blade) or "dedicated" (1 BMC per node)

### Rack
Physical rack in a data center:
- References a RackTemplate via `templateUID`
- Contains location information
- Status tracks:
  - Provisioning phase (Pending → Provisioning → Ready)
  - Child resource UIDs
  - Total counts of chassis, blades, nodes, BMCs

### Chassis
Blade chassis container:
- Belongs to a Rack
- Contains Blades
- Tracks chassis number and health

### Blade
Blade server:
- Belongs to a Chassis
- Contains Nodes and BMCs
- Tracks blade number and power state

### BMC
Baseboard Management Controller:
- Belongs to a Blade
- Manages 1-8 Nodes
- Tracks IP address, reachability, and health

### Node
Compute node:
- Belongs to a Blade
- Managed by a BMC
- Tracks power state, boot state, and hardware configuration

## Reconciliation Workflow

When a Rack is created with a RackTemplate reference:

1. **RackReconciler** is triggered
2. Reconciler loads the referenced RackTemplate
3. For each chassis specified in the template:
   - Creates Chassis resource
   - For each blade in the chassis:
     - Creates Blade resource
     - Creates BMC resource(s) based on mode
     - For each node in the blade:
       - Creates Node resource
       - Associates with BMC
4. Updates Rack status with:
   - Phase = "Ready"
   - Child resource UIDs
   - Total resource counts

## Integration Test

### Test Structure

The integration test (`rack_reconciliation_simple_test.go`) verifies:

1. ✅ Project initialization with file storage
2. ✅ Addition of all resource types
3. ✅ Resource definition with proper Spec/Status structs
4. ✅ Code generation for all resources
5. ✅ Reconciler implementation compiles correctly
6. ✅ Complete project builds successfully

### Running the Test

```bash
cd test/integration
go test -v -run TestRackReconciliationSimpleTestSuite
```

Expected output:
```
=== RUN   TestRackReconciliationSimpleTestSuite/TestRackReconciliationCodeGeneration
    ✓ Rack reconciliation code generation test passed
    ✓ All resources defined correctly
    ✓ Reconciler compiles successfully
    ✓ Project builds with reconciliation support
--- PASS: TestRackReconciliationSimpleTestSuite
```

## Implementation Details

### RackReconciler

The RackReconciler (`rack_reconciler.go`) implements the `reconcile.Reconciler` interface:

```go
type RackReconciler struct {
    reconcile.BaseReconciler
    Storage storage.StorageBackend
}

func (r *RackReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
    // 1. Load RackTemplate
    // 2. Create child resources based on template
    // 3. Update Rack status
    // 4. Return requeue interval
}
```

**Key Features:**
- Idempotent reconciliation (safe to call multiple times)
- Proper error handling with phase tracking
- Hierarchical resource creation
- Status updates with resource counts

### Storage Integration

The reconciler uses Fabrica's storage backend:
- Loads templates via `storage.Load()`
- Saves resources via `storage.Save()` (with JSON marshaling)
- Updates resource status atomically

### Event Integration

The reconciler is designed to work with Fabrica's event system:
- Controller subscribes to resource events
- Rack creation triggers reconciliation
- Reconciler emits completion events

## Use Cases

This pattern is useful for:

1. **Data Center Infrastructure**: Racks, chassis, servers, nodes
2. **Cluster Management**: Clusters, node pools, nodes
3. **Network Topology**: Racks, switches, ports, connections
4. **Multi-Tenant Systems**: Tenants, projects, resources
5. **Hierarchical Configurations**: Templates → Instances → Components

## Future Enhancements

Potential improvements:

1. **State Machine Integration**: Add go-workflows or Temporal for complex workflows
2. **Validation**: Validate template constraints (e.g., max nodes per blade)
3. **Cascade Deletion**: Automatically delete child resources when parent is deleted
4. **Status Aggregation**: Roll up child resource health to parent
5. **Partial Updates**: Support updating templates and propagating changes
6. **Reconciliation Strategies**: Different strategies for different resource types

## Related Documentation

- [Reconciliation Framework](../../../docs/reconciliation.md)
- [Events System](../../../docs/events.md)
- [Storage Backends](../../../docs/storage.md)
- [Resource Model](../../../docs/resource-model.md)
