// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for Rack.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
//
// This example demonstrates a complete rack reconciliation implementation
// that creates hierarchical infrastructure resources based on a template.
package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/example/rack-inventory/pkg/resources/blade"
	"github.com/example/rack-inventory/pkg/resources/bmc"
	"github.com/example/rack-inventory/pkg/resources/chassis"
	"github.com/example/rack-inventory/pkg/resources/node"
	"github.com/example/rack-inventory/pkg/resources/rack"
	"github.com/example/rack-inventory/pkg/resources/racktemplate"
	"github.com/openchami/fabrica/pkg/resource"
)

// reconcileRack implements the custom business logic for Rack reconciliation.
// This is where you implement the actual reconciliation logic.
func (r *RackReconciler) reconcileRack(ctx context.Context, rackResource *rack.Rack) error {
	// Check if already provisioned (idempotency)
	if rackResource.Status.Phase == "Ready" {
		r.Logger.Debugf("Rack %s already in Ready phase", rackResource.GetUID())
		return nil
	}

	// Update phase to Provisioning
	if rackResource.Status.Phase != "Provisioning" {
		rackResource.Status.Phase = "Provisioning"
		if err := r.Client.Update(ctx, rackResource); err != nil {
			return fmt.Errorf("failed to update rack status: %w", err)
		}
	}

	// Load the RackTemplate
	templateUID := rackResource.Spec.TemplateUID
	templateData, err := r.Client.Get(ctx, "RackTemplate", templateUID)
	if err != nil {
		r.Logger.Errorf("Failed to load RackTemplate %s: %v", templateUID, err)
		rackResource.Status.Phase = "Error"
		r.Client.Update(ctx, rackResource)
		return fmt.Errorf("failed to load rack template: %w", err)
	}

	// Unmarshal template
	var template racktemplate.RackTemplate
	templateBytes, err := json.Marshal(templateData)
	if err != nil {
		return fmt.Errorf("failed to marshal template data: %w", err)
	}
	if err := json.Unmarshal(templateBytes, &template); err != nil {
		return fmt.Errorf("failed to unmarshal template: %w", err)
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
			r.Client.Update(ctx, rackResource)
			return err
		}
		chassisUIDs = append(chassisUIDs, chassisUID)

		// Create Blades in this Chassis
		bladeUIDs := []string{}
		for bladeNum := 0; bladeNum < template.Spec.ChassisConfig.BladeCount; bladeNum++ {
			bladeUID, err := r.createBlade(ctx, chassisUID, bladeNum)
			if err != nil {
				r.Logger.Errorf("Failed to create blade %d in chassis %d: %v", bladeNum, chassisNum, err)
				return err
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
					return err
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
						return err
					}
					bmcUIDs = append(bmcUIDs, bmcUID)
					totalBMCs++
				}

				nodeUID, err := r.createNode(ctx, bladeUID, bmcUID, nodeNum)
				if err != nil {
					r.Logger.Errorf("Failed to create node %d in blade %s: %v", nodeNum, bladeUID, err)
					return err
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

	if err := r.Client.Update(ctx, rackResource); err != nil {
		return fmt.Errorf("failed to update rack status: %w", err)
	}

	r.Logger.Infof("Rack %s provisioned: %d chassis, %d blades, %d nodes, %d BMCs",
		rackResource.GetName(), len(chassisUIDs), totalBlades, totalNodes, totalBMCs)

	return nil
}

// createChassis creates a new Chassis resource
func (r *RackReconciler) createChassis(ctx context.Context, rackResource *rack.Rack, chassisNum int) (string, error) {
	chassisUID, err := resource.GenerateUIDForResource("Chassis")
	if err != nil {
		return "", err
	}

	c := &chassis.Chassis{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "Chassis",
			Metadata: resource.Metadata{
				Name: fmt.Sprintf("%s-chassis-%d", rackResource.GetName(), chassisNum),
				UID:  chassisUID,
			},
		},
		Spec: chassis.ChassisSpec{
			RackUID:       rackResource.GetUID(),
			ChassisNumber: chassisNum,
		},
		Status: chassis.ChassisStatus{
			PowerState: "Unknown",
			Health:     "Unknown",
		},
	}
	c.Metadata.Initialize(c.Metadata.Name, c.Metadata.UID)

	if err := r.Client.Update(ctx, c); err != nil {
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

	b := &blade.Blade{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "Blade",
			Metadata: resource.Metadata{
				Name: fmt.Sprintf("chassis-%s-blade-%d", chassisUID, bladeNum),
				UID:  bladeUID,
			},
		},
		Spec: blade.BladeSpec{
			ChassisUID:  chassisUID,
			BladeNumber: bladeNum,
		},
		Status: blade.BladeStatus{
			PowerState: "Unknown",
			Health:     "Unknown",
		},
	}
	b.Metadata.Initialize(b.Metadata.Name, b.Metadata.UID)

	if err := r.Client.Update(ctx, b); err != nil {
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

	b := &bmc.BMC{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "BMC",
			Metadata: resource.Metadata{
				Name: fmt.Sprintf("bmc-%s", bmcUID),
				UID:  bmcUID,
			},
		},
		Spec: bmc.BMCSpec{
			BladeUID: bladeUID,
		},
		Status: bmc.BMCStatus{
			Reachable: false,
			Health:    "Unknown",
		},
	}
	b.Metadata.Initialize(b.Metadata.Name, b.Metadata.UID)

	if err := r.Client.Update(ctx, b); err != nil {
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

	n := &node.Node{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "Node",
			Metadata: resource.Metadata{
				Name: fmt.Sprintf("node-%s-%d", bladeUID, nodeNum),
				UID:  nodeUID,
			},
		},
		Spec: node.NodeSpec{
			BladeUID:   bladeUID,
			BMCUID:     bmcUID,
			NodeNumber: nodeNum,
		},
		Status: node.NodeStatus{
			PowerState: "Unknown",
			BootState:  "Unknown",
			Health:     "Unknown",
		},
	}
	n.Metadata.Initialize(n.Metadata.Name, n.Metadata.UID)

	if err := r.Client.Update(ctx, n); err != nil {
		return "", fmt.Errorf("failed to save node: %w", err)
	}

	r.Logger.Debugf("Created Node %s", nodeUID)
	return nodeUID, nil
}

// updateChassisStatus updates the chassis with blade UIDs
func (r *RackReconciler) updateChassisStatus(ctx context.Context, chassisUID string, bladeUIDs []string) error {
	data, err := r.Client.Get(ctx, "Chassis", chassisUID)
	if err != nil {
		return err
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var c chassis.Chassis
	if err := json.Unmarshal(dataBytes, &c); err != nil {
		return err
	}

	c.Status.BladeUIDs = bladeUIDs
	return r.Client.Update(ctx, &c)
}

// updateBladeStatus updates the blade with node and BMC UIDs
func (r *RackReconciler) updateBladeStatus(ctx context.Context, bladeUID string, nodeUIDs, bmcUIDs []string) error {
	data, err := r.Client.Get(ctx, "Blade", bladeUID)
	if err != nil {
		return err
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var b blade.Blade
	if err := json.Unmarshal(dataBytes, &b); err != nil {
		return err
	}

	b.Status.NodeUIDs = nodeUIDs
	b.Status.BMCUIDs = bmcUIDs
	return r.Client.Update(ctx, &b)
}
