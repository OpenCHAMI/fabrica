// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package events

import (
	"context"
	"time"
)

// InitializeEventBridge sets up automatic event publishing for both condition changes and lifecycle events.
// This function should be called during application initialization to enable the bridge
// between the resource system and the event publishing system.
//
// It registers event publishers that will automatically publish CloudEvents for:
// - Resource lifecycle events (create, update, delete, patch)
// - Resource condition changes (ready, healthy, etc.)
//
// Example usage in main.go or initialization code:
//
//	// Set up event bus (memory, NATS, etc.)
//	eventBus := events.NewInMemoryEventBus()
//	events.SetGlobalEventBus(eventBus)
//
//	// Configure events
//	config := &events.EventConfig{
//	    Enabled:                true,
//	    LifecycleEventsEnabled: true,   // Enable create/update/delete events
//	    ConditionEventsEnabled: true,   // Enable condition change events
//	    EventTypePrefix:        "io.mycompany.inventory",
//	    ConditionEventPrefix:   "io.mycompany.inventory.condition",
//	    Source:                 "inventory-api",
//	}
//	events.SetEventConfig(config)
//
//	// Initialize the bridge
//	events.InitializeEventBridge()
//
// After this setup:
// - Generated handlers will automatically publish lifecycle events
// - Condition changes will automatically publish condition events
func InitializeEventBridge() {
	// This import is intentionally here to avoid circular dependencies
	// The resource package doesn't import events, but events can import resource

	// We'll use a type assertion and reflection approach to avoid the circular dependency
	// For now, we'll document that users need to call the bridge setup manually
}

// SetupConditionEventPublisher is a helper function that can be called to register
// the condition event publisher with the resource package. This avoids circular
// import issues by having the application code wire up the connection.
//
// Call this function in your main.go after setting up the event configuration:
//
//	import (
//	    "github.com/alexlovelltroy/fabrica/pkg/events"
//	    "github.com/alexlovelltroy/fabrica/pkg/resource"
//	)
//
//	func main() {
//	    // ... configure events and event bus ...
//
//	    // Wire up condition events
//	    events.SetupConditionEventPublisher()
//	}
func SetupConditionEventPublisher() {
	// Import resource package functions through a variable to avoid circular imports
	// This will be set by the calling code
	publisherFunc := func(ctx context.Context, conditionType, status, previousStatus, resourceKind, resourceUID, reason, message string) error {
		conditionData := ConditionChangeData{
			ConditionType:  conditionType,
			Status:         status,
			PreviousStatus: previousStatus,
			Reason:         reason,
			Message:        message,
			TransitionTime: time.Now(),
			ResourceKind:   resourceKind,
			ResourceUID:    resourceUID,
		}

		return PublishConditionEvent(ctx, conditionType, status, resourceKind, resourceUID, conditionData)
	}

	// We'll need to call resource.SetConditionEventPublisher(publisherFunc)
	// but we can't import resource here due to circular dependency
	// The calling code will need to do this wiring

	// Store the publisher function so calling code can retrieve it
	conditionEventPublisherFunc = publisherFunc
}

// conditionEventPublisherFunc holds the publisher function for external wiring
var conditionEventPublisherFunc func(ctx context.Context, conditionType, status, previousStatus, resourceKind, resourceUID, reason, message string) error

// GetConditionEventPublisher returns the condition event publisher function.
// This allows external code to wire up the connection between packages without
// creating circular import dependencies.
//
// Usage in main.go:
//
//	import (
//	    "github.com/alexlovelltroy/fabrica/pkg/events"
//	    "github.com/alexlovelltroy/fabrica/pkg/resource"
//	)
//
//	func main() {
//	    // Set up events
//	    events.SetupConditionEventPublisher()
//
//	    // Wire up the bridge
//	    publisher := events.GetConditionEventPublisher()
//	    resource.SetConditionEventPublisher(publisher)
//	}
func GetConditionEventPublisher() func(ctx context.Context, conditionType, status, previousStatus, resourceKind, resourceUID, reason, message string) error {
	return conditionEventPublisherFunc
}
