// SPDX-FileCopyrightText: 2025 Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package testdata

// RackReconcilerCode provides the reconciler implementation for Rack resources in versioned layout
const RackReconcilerCode = `// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package reconciler

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/openchami/fabrica/pkg/events"
    "github.com/openchami/fabrica/pkg/fabrica"
    "github.com/openchami/fabrica/pkg/reconcile"
    "github.com/openchami/fabrica/pkg/resource"
    "github.com/openchami/fabrica/pkg/storage"

    v1 "` + "{{.Module}}" + `/apis/example.com/v1"
)

const apiVersion = "example.com/v1"

// RackReconciler handles reconciliation of Rack resources
type RackReconciler struct {
    reconcile.BaseReconciler
    Storage storage.StorageBackend
}

// NewRackReconciler creates a new RackReconciler
func NewRackReconciler(storage storage.StorageBackend, eventBus events.EventBus) *RackReconciler {
    return &RackReconciler{
        BaseReconciler: reconcile.BaseReconciler{
            EventBus: eventBus,
            Logger:   reconcile.NewDefaultLogger(),
        },
        Storage: storage,
    }
}

// Reconcile brings the Rack to its desired state by creating child resources
func (r *RackReconciler) Reconcile(ctx context.Context, resourceInterface interface{}) (reconcile.Result, error) {
    rackResource, ok := resourceInterface.(*v1.Rack)
    if !ok {
        return reconcile.Result{}, fmt.Errorf("expected *v1.Rack, got %T", resourceInterface)
    }

    r.Logger.Infof("Reconciling Rack %s (UID: %s)", rackResource.GetName(), rackResource.GetUID())

    // Check if already provisioned
    if rackResource.Status.Phase == "Ready" {
        r.Logger.Debugf("Rack %s already in Ready phase", rackResource.GetUID())
        return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
    }

    // Update phase to Provisioning
    if rackResource.Status.Phase != "Provisioning" {
        rackResource.Status.Phase = "Provisioning"
        if err := r.saveResource(ctx, rackResource.GetKind(), rackResource.GetUID(), rackResource); err != nil {
            return reconcile.Result{}, fmt.Errorf("failed to update rack status: %w", err)
        }
    }

    // Load the RackTemplate
    templateUID := rackResource.Spec.TemplateUID
    templateData, err := r.Storage.Load(ctx, "RackTemplate", templateUID)
    if err != nil {
        r.Logger.Errorf("Failed to load RackTemplate %s: %v", templateUID, err)
        rackResource.Status.Phase = "Error"
        r.saveResource(ctx, rackResource.GetKind(), rackResource.GetUID(), rackResource)
        return reconcile.Result{}, fmt.Errorf("failed to load rack template: %w", err)
    }

    // Unmarshal template
    var template v1.RackTemplate
    templateBytes, err := json.Marshal(templateData)
    if err != nil {
        return reconcile.Result{}, fmt.Errorf("failed to marshal template data: %w", err)
    }
    if err := json.Unmarshal(templateBytes, &template); err != nil {
        return reconcile.Result{}, fmt.Errorf("failed to unmarshal template: %w", err)
    }

    r.Logger.Infof("Using RackTemplate %s: %d chassis, %d blades per chassis, %d nodes per blade",
        template.GetName(),
        template.Spec.ChassisCount,
        template.Spec.ChassisConfig.BladeCount,
        template.Spec.ChassisConfig.BladeConfig.NodeCount)

    // Create chassis, blades, nodes, and BMCs
    chassisUIDs := []string{}
    totalBlades := 0
    totalNodes := 0
    totalBMCs := 0

    for chassisNum := 0; chassisNum < template.Spec.ChassisCount; chassisNum++ {
        // Create Chassis
        chassisUID, err := r.createChassis(ctx, rackResource, chassisNum)
        if err != nil {
            r.Logger.Errorf("Failed to create chassis %d: %v", chassisNum, err)
            rackResource.Status.Phase = "Error"
            r.saveResource(ctx, rackResource.GetKind(), rackResource.GetUID(), rackResource)
            return reconcile.Result{}, err
        }
        chassisUIDs = append(chassisUIDs, chassisUID)

        // Create Blades in this Chassis
        bladeUIDs := []string{}
        for bladeNum := 0; bladeNum < template.Spec.ChassisConfig.BladeCount; bladeNum++ {
            bladeUID, err := r.createBlade(ctx, chassisUID, bladeNum)
            if err != nil {
                r.Logger.Errorf("Failed to create blade %d in chassis %d: %v", bladeNum, chassisNum, err)
                return reconcile.Result{}, err
            }
            bladeUIDs = append(bladeUIDs, bladeUID)
            totalBlades++

            // Create BMC(s) for this Blade
            bmcUIDs := []string{}
            if template.Spec.ChassisConfig.BladeConfig.BMCMode == "shared" {
                // One BMC per blade
                bmcUID, err := r.createBMC(ctx, bladeUID)
                if err != nil {
                    r.Logger.Errorf("Failed to create BMC for blade %s: %v", bladeUID, err)
                    return reconcile.Result{}, err
                }
                bmcUIDs = append(bmcUIDs, bmcUID)
                totalBMCs++
            }

            // Create Nodes in this Blade
            nodeUIDs := []string{}
            for nodeNum := 0; nodeNum < template.Spec.ChassisConfig.BladeConfig.NodeCount; nodeNum++ {
                var bmcUID string
                if template.Spec.ChassisConfig.BladeConfig.BMCMode == "shared" {
                    bmcUID = bmcUIDs[0]
                } else {
                    // Dedicated mode: create one BMC per node
                    var err error
                    bmcUID, err = r.createBMC(ctx, bladeUID)
                    if err != nil {
                        r.Logger.Errorf("Failed to create dedicated BMC for node %d: %v", nodeNum, err)
                        return reconcile.Result{}, err
                    }
                    bmcUIDs = append(bmcUIDs, bmcUID)
                    totalBMCs++
                }

                nodeUID, err := r.createNode(ctx, bladeUID, bmcUID, nodeNum)
                if err != nil {
                    r.Logger.Errorf("Failed to create node %d in blade %s: %v", nodeNum, bladeUID, err)
                    return reconcile.Result{}, err
                }
                nodeUIDs = append(nodeUIDs, nodeUID)
                totalNodes++
            }

            // Update blade with node and BMC UIDs
            r.updateBladeStatus(ctx, bladeUID, nodeUIDs, bmcUIDs)
        }

        // Update chassis with blade UIDs
        r.updateChassisStatus(ctx, chassisUID, bladeUIDs)
    }

    // Update Rack status
    rackResource.Status.Phase = "Ready"
    rackResource.Status.ChassisUIDs = chassisUIDs
    rackResource.Status.TotalChassis = len(chassisUIDs)
    rackResource.Status.TotalBlades = totalBlades
    rackResource.Status.TotalNodes = totalNodes
    rackResource.Status.TotalBMCs = totalBMCs

    if err := r.saveResource(ctx, rackResource.GetKind(), rackResource.GetUID(), rackResource); err != nil {
        return reconcile.Result{}, fmt.Errorf("failed to update rack status: %w", err)
    }

    r.Logger.Infof("Rack %s provisioned: %d chassis, %d blades, %d nodes, %d BMCs",
        rackResource.GetName(), totalBlades, totalNodes, totalBMCs)

    // Emit completion event
    r.EmitEvent(ctx, "io.fabrica.rack.provisioned", rackResource)

    return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
}

// createChassis creates a new Chassis resource
func (r *RackReconciler) createChassis(ctx context.Context, rackResource *v1.Rack, chassisNum int) (string, error) {
    chassisUID, err := resource.GenerateUIDForResource("Chassis")
    if err != nil {
        return "", err
    }

    c := &v1.Chassis{
        APIVersion: apiVersion,
        Kind:       "Chassis",
        Metadata: fabrica.Metadata{
            Name: fmt.Sprintf("%s-chassis-%d", rackResource.GetName(), chassisNum),
            UID:  chassisUID,
        },
        Spec: v1.ChassisSpec{
            RackUID:       rackResource.GetUID(),
            ChassisNumber: chassisNum,
        },
        Status: v1.ChassisStatus{
            PowerState: "Unknown",
            Health:     "Unknown",
        },
    }

    if err := r.saveResource(ctx, c.GetKind(), c.GetUID(), c); err != nil {
        return "", fmt.Errorf("failed to save chassis: %w", err)
    }

    r.Logger.Debugf("Created Chassis %s", chassisUID)
    return chassisUID, nil
}

// createBlade creates a new Blade resource
func (r *RackReconciler) createBlade(ctx context.Context, chassisUID string, bladeNum int) (string, error) {
    bladeUID, err := resource.GenerateUIDForResource("Blade")
    if err != nil {
        return "", err
    }

    b := &v1.Blade{
        APIVersion: apiVersion,
        Kind:       "Blade",
        Metadata: fabrica.Metadata{
            Name: fmt.Sprintf("chassis-%d-blade-%d", bladeNum, bladeNum),
            UID:  bladeUID,
        },
        Spec: v1.BladeSpec{
            ChassisUID:  chassisUID,
            BladeNumber: bladeNum,
        },
        Status: v1.BladeStatus{
            PowerState: "Unknown",
            Health:     "Unknown",
        },
    }

    if err := r.saveResource(ctx, b.GetKind(), b.GetUID(), b); err != nil {
        return "", fmt.Errorf("failed to save blade: %w", err)
    }

    r.Logger.Debugf("Created Blade %s", bladeUID)
    return bladeUID, nil
}

// createBMC creates a new BMC resource
func (r *RackReconciler) createBMC(ctx context.Context, bladeUID string) (string, error) {
    bmcUID, err := resource.GenerateUIDForResource("BMC")
    if err != nil {
        return "", err
    }

    b := &v1.BMC{
        APIVersion: apiVersion,
        Kind:       "BMC",
        Metadata: fabrica.Metadata{
            Name: fmt.Sprintf("bmc-%s", bmcUID),
            UID:  bmcUID,
        },
        Spec: v1.BMCSpec{
            BladeUID: bladeUID,
        },
        Status: v1.BMCStatus{
            Reachable: false,
            Health:    "Unknown",
        },
    }

    if err := r.saveResource(ctx, b.GetKind(), b.GetUID(), b); err != nil {
        return "", fmt.Errorf("failed to save BMC: %w", err)
    }

    r.Logger.Debugf("Created BMC %s", bmcUID)
    return bmcUID, nil
}

// createNode creates a new Node resource
func (r *RackReconciler) createNode(ctx context.Context, bladeUID, bmcUID string, nodeNum int) (string, error) {
    nodeUID, err := resource.GenerateUIDForResource("Node")
    if err != nil {
        return "", err
    }

    n := &v1.Node{
        APIVersion: apiVersion,
        Kind:       "Node",
        Metadata: fabrica.Metadata{
            Name: fmt.Sprintf("node-%s-%d", bladeUID, nodeNum),
            UID:  nodeUID,
        },
        Spec: v1.NodeSpec{
            BladeUID:   bladeUID,
            BMCUID:     bmcUID,
            NodeNumber: nodeNum,
        },
        Status: v1.NodeStatus{
            PowerState: "Unknown",
            BootState:  "Unknown",
            Health:     "Unknown",
        },
    }

    if err := r.saveResource(ctx, n.GetKind(), n.GetUID(), n); err != nil {
        return "", fmt.Errorf("failed to save node: %w", err)
    }

    r.Logger.Debugf("Created Node %s", nodeUID)
    return nodeUID, nil
}

// updateChassisStatus updates the chassis with blade UIDs
func (r *RackReconciler) updateChassisStatus(ctx context.Context, chassisUID string, bladeUIDs []string) error {
    data, err := r.Storage.Load(ctx, "Chassis", chassisUID)
    if err != nil {
        return err
    }

    dataBytes, err := json.Marshal(data)
    if err != nil {
        return err
    }

    var c v1.Chassis
    if err := json.Unmarshal(dataBytes, &c); err != nil {
        return err
    }

    c.Status.BladeUIDs = bladeUIDs
    return r.saveResource(ctx, c.GetKind(), c.GetUID(), &c)
}

// updateBladeStatus updates the blade with node and BMC UIDs
func (r *RackReconciler) updateBladeStatus(ctx context.Context, bladeUID string, nodeUIDs, bmcUIDs []string) error {
    data, err := r.Storage.Load(ctx, "Blade", bladeUID)
    if err != nil {
        return err
    }

    dataBytes, err := json.Marshal(data)
    if err != nil {
        return err
    }

    var b v1.Blade
    if err := json.Unmarshal(dataBytes, &b); err != nil {
        return err
    }

    b.Status.NodeUIDs = nodeUIDs
    b.Status.BMCUIDs = bmcUIDs
    return r.saveResource(ctx, b.GetKind(), b.GetUID(), &b)
}

// GetResourceKind returns the resource kind this reconciler handles
func (r *RackReconciler) GetResourceKind() string {
    return "Rack"
}

// saveResource marshals and saves a resource to storage
func (r *RackReconciler) saveResource(ctx context.Context, kind, uid string, res interface{}) error {
    data, err := json.Marshal(res)
    if err != nil {
        return fmt.Errorf("failed to marshal resource: %w", err)
    }
    return r.Storage.Save(ctx, kind, uid, data)
}
`
