// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
//
// Example: How to set up and use the Conditions + Events bridge system
//
// This example shows how to configure Fabrica to automatically publish CloudEvents
// whenever resource conditions change, implementing the Option 1 (Bridge Pattern)
// approach discussed.

package main

import (
	"context"
	"log"

	"github.com/alexlovelltroy/fabrica/pkg/events"
	"github.com/alexlovelltroy/fabrica/pkg/resource"
)

func main() {
	// Step 1: Configure event publishing
	eventConfig := &events.EventConfig{
		Enabled:                true, // Enable event publishing
		LifecycleEventsEnabled: true, // Enable create/update/delete events
		ConditionEventsEnabled: true, // Enable condition change events
		EventTypePrefix:        "io.mycompany.inventory",
		ConditionEventPrefix:   "io.mycompany.inventory.condition",
		Source:                 "inventory-api",
	}
	events.SetEventConfig(eventConfig)

	// Step 2: Set up event bus (this would be your chosen implementation)
	// eventBus := events.NewMemoryEventBus()  // or NATS, Kafka, etc.
	// events.SetGlobalEventBus(eventBus)

	// Step 3: Wire up condition event publishing
	events.SetupConditionEventPublisher()
	publisher := events.GetConditionEventPublisher()
	resource.SetConditionEventPublisher(publisher)

	// Step 4: Use conditions normally - events happen automatically!
	ctx := context.Background()

	// Example Device resource (this would be your actual resource type)
	device := &ExampleDevice{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "Device",
		},
		Status: ExampleDeviceStatus{
			Conditions: []resource.Condition{},
		},
	}
	device.SetUID("dev-123456")
	device.SetName("temperature-sensor-01")

	// Option 1: Use the helper function (recommended)
	// This automatically publishes events
	changed := resource.SetResourceCondition(ctx, device, "Ready", "True", "Healthy", "All systems operational")
	if changed {
		log.Println("Device condition updated and event published!")
	}

	// Option 2: Use SetConditionWithEvents directly
	// This also publishes events
	resource.SetConditionWithEvents(ctx, &device.Status.Conditions, "Reachable", "False", "NetworkTimeout", "Device not responding to ping", "Device", device.GetUID())

	// Option 3: Use regular SetCondition (no events)
	// This does NOT publish events - for when you don't want events
	resource.SetCondition(&device.Status.Conditions, "InternalFlag", "True", "Testing", "Internal condition for testing")

	// Step 5: Demonstrate lifecycle event publishing

	// Publish resource creation event (typically called by generated handlers)
	err := events.PublishResourceCreated(ctx, "Device", device.GetUID(), device.GetName(), device)
	if err != nil {
		log.Printf("Failed to publish created event: %v", err)
	}

	// Simulate an update with metadata
	updateMetadata := map[string]interface{}{
		"updatedFields": []string{"spec.location"},
		"updatedBy":     "api-user",
		"reason":        "location changed",
	}
	err = events.PublishResourceUpdated(ctx, "Device", device.GetUID(), device.GetName(), device, updateMetadata)
	if err != nil {
		log.Printf("Failed to publish updated event: %v", err)
	}

	// Demonstrate patch event (for partial updates)
	patchData := map[string]interface{}{
		"op":    "replace",
		"path":  "/spec/location",
		"value": "warehouse-b",
	}
	err = events.PublishResourcePatched(ctx, "Device", device.GetUID(), device.GetName(), device, patchData)
	if err != nil {
		log.Printf("Failed to publish patched event: %v", err)
	}

	// The generated event types would be:
	// Condition Events:
	// - "io.mycompany.inventory.condition.ready" (for Ready condition)
	// - "io.mycompany.inventory.condition.reachable" (for Reachable condition)
	// - No event for InternalFlag (used SetCondition without events)
	//
	// Lifecycle Events:
	// - "io.mycompany.inventory.device.created" (resource creation)
	// - "io.mycompany.inventory.device.updated" (resource update)
	// - "io.mycompany.inventory.device.patched" (resource patch)
	// - "io.mycompany.inventory.device.deleted" (resource deletion)

	log.Println("Example completed successfully! Events published for conditions and lifecycle operations.")
}

// ExampleDevice shows how a resource type would implement ResourceWithConditions
type ExampleDevice struct {
	resource.Resource `json:",inline"`
	Spec              ExampleDeviceSpec   `json:"spec"`
	Status            ExampleDeviceStatus `json:"status"`
}

type ExampleDeviceSpec struct {
	IPAddress string `json:"ipAddress"`
	Location  string `json:"location"`
}

type ExampleDeviceStatus struct {
	Conditions []resource.Condition `json:"conditions"`
	Health     string               `json:"health"`
}

// Implement ResourceWithConditions interface
func (d *ExampleDevice) GetUID() string {
	return d.Resource.GetUID()
}

func (d *ExampleDevice) GetKind() string {
	return d.Kind
}

func (d *ExampleDevice) GetConditions() *[]resource.Condition {
	return &d.Status.Conditions
}

// Generated code would include methods like these:
func (d *ExampleDevice) SetReadyCondition(ctx context.Context, status, reason, message string) {
	resource.SetResourceCondition(ctx, d, "Ready", status, reason, message)
}

func (d *ExampleDevice) SetHealthyCondition(ctx context.Context, status, reason, message string) {
	resource.SetResourceCondition(ctx, d, "Healthy", status, reason, message)
}

func (d *ExampleDevice) IsReady() bool {
	return resource.IsConditionTrue(d.Status.Conditions, "Ready")
}

func (d *ExampleDevice) IsHealthy() bool {
	return resource.IsConditionTrue(d.Status.Conditions, "Healthy")
}
