// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package events provides a CloudEvents-based event system for inventory resources.
//
// This package implements event publishing and subscription using the CloudEvents
// standard (https://cloudevents.io/), enabling interoperability with external systems
// and cloud-native event tooling.
//
// Events can be gated behind feature flags and configured with custom prefixes
// for different deployment environments and integration patterns.
package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// EventConfig controls event publishing behavior and prefixes
type EventConfig struct {
	// Enabled controls whether events are published at all
	Enabled bool `json:"enabled" yaml:"enabled"`

	// LifecycleEventsEnabled controls automatic resource lifecycle events (create, update, delete)
	LifecycleEventsEnabled bool `json:"lifecycleEventsEnabled" yaml:"lifecycleEventsEnabled"`

	// ConditionEventsEnabled controls automatic condition change events
	ConditionEventsEnabled bool `json:"conditionEventsEnabled" yaml:"conditionEventsEnabled"`

	// EventTypePrefix sets the prefix for all generated event types
	// Example: "io.fabrica" generates "io.fabrica.device.created"
	EventTypePrefix string `json:"eventTypePrefix" yaml:"eventTypePrefix"`

	// ConditionEventPrefix sets the prefix for condition change events
	// Example: "io.fabrica.condition" generates "io.fabrica.condition.ready"
	ConditionEventPrefix string `json:"conditionEventPrefix" yaml:"conditionEventPrefix"`

	// Source sets the default source identifier for events
	// Example: "fabrica-api" or "inventory-system"
	Source string `json:"source" yaml:"source"`
}

// DefaultEventConfig returns sensible defaults for event configuration
func DefaultEventConfig() *EventConfig {
	return &EventConfig{
		Enabled:                false, // Disabled by default - must be explicitly enabled
		LifecycleEventsEnabled: true,  // Enable lifecycle events when events are enabled
		ConditionEventsEnabled: true,  // Enable condition events when events are enabled
		EventTypePrefix:        "io.fabrica",
		ConditionEventPrefix:   "io.fabrica.condition",
		Source:                 "fabrica-api",
	}
}

// GlobalEventConfig holds the runtime configuration for events
var globalEventConfig = DefaultEventConfig()
var configMutex sync.RWMutex

// SetEventConfig updates the global event configuration at runtime
func SetEventConfig(config *EventConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalEventConfig = config
}

// GetEventConfig returns the current global event configuration
func GetEventConfig() *EventConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()

	// Return a copy to prevent external modification
	return &EventConfig{
		Enabled:                globalEventConfig.Enabled,
		LifecycleEventsEnabled: globalEventConfig.LifecycleEventsEnabled,
		ConditionEventsEnabled: globalEventConfig.ConditionEventsEnabled,
		EventTypePrefix:        globalEventConfig.EventTypePrefix,
		ConditionEventPrefix:   globalEventConfig.ConditionEventPrefix,
		Source:                 globalEventConfig.Source,
	}
}

// IsEnabled returns true if event publishing is enabled
func IsEnabled() bool {
	return GetEventConfig().Enabled
}

// AreLifecycleEventsEnabled returns true if resource lifecycle events are enabled
func AreLifecycleEventsEnabled() bool {
	config := GetEventConfig()
	return config.Enabled && config.LifecycleEventsEnabled
}

// AreConditionEventsEnabled returns true if condition change events are enabled
func AreConditionEventsEnabled() bool {
	config := GetEventConfig()
	return config.Enabled && config.ConditionEventsEnabled
}

// Event wraps CloudEvents specification
type Event struct {
	cloudevents.Event
}

// NewEvent creates a CloudEvents-compliant event
func NewEvent(eventType, source string, data interface{}) (*Event, error) {
	event := cloudevents.NewEvent()
	event.SetID(generateEventID())
	event.SetType(eventType)
	event.SetSource(source)
	event.SetTime(time.Now())
	event.SetDataContentType("application/json")

	if err := event.SetData(cloudevents.ApplicationJSON, data); err != nil {
		return nil, fmt.Errorf("failed to set event data: %w", err)
	}

	return &Event{Event: event}, nil
}

// NewResourceEvent creates an event for a resource change using configured prefixes
//
// Parameters:
//   - action: The action that occurred (e.g., "created", "updated", "deleted")
//   - resourceKind: Kind of resource (e.g., "Device", "User")
//   - resourceUID: Unique identifier of the resource
//   - data: Event payload data
//
// Returns:
//   - *Event: CloudEvents-compliant event with proper prefix
//   - error: If event creation fails or events are disabled
//
// Example:
//
//	event, err := NewResourceEvent("created", "Device", "dev-123", deviceData)
//	// Generates event type: "io.fabrica.device.created"
func NewResourceEvent(action, resourceKind, resourceUID string, data interface{}) (*Event, error) {
	// Check if lifecycle events are enabled for create/update/delete actions
	lifecycleActions := map[string]bool{
		"created": true, "create": true,
		"updated": true, "update": true,
		"deleted": true, "delete": true,
		"patched": true, "patch": true,
	}

	if lifecycleActions[strings.ToLower(action)] && !AreLifecycleEventsEnabled() {
		return nil, fmt.Errorf("lifecycle events are disabled")
	}

	if !IsEnabled() {
		return nil, fmt.Errorf("events are disabled")
	}

	config := GetEventConfig()

	// Build event type with configured prefix: prefix.resourcekind.action
	eventType := fmt.Sprintf("%s.%s.%s",
		config.EventTypePrefix,
		strings.ToLower(resourceKind),
		strings.ToLower(action))

	source := fmt.Sprintf("%s/resources/%s/%s", config.Source, resourceKind, resourceUID)
	event, err := NewEvent(eventType, source, data)
	if err != nil {
		return nil, err
	}

	// Add resource-specific extension attributes
	event.SetExtension("resourcekind", resourceKind)
	event.SetExtension("resourceuid", resourceUID)
	event.SetExtension("action", action)

	return event, nil
}

// NewConditionEvent creates an event for a resource condition change
//
// Parameters:
//   - conditionType: The type of condition (e.g., "Ready", "Healthy")
//   - status: The new condition status ("True", "False", "Unknown")
//   - resourceKind: Kind of resource (e.g., "Device", "User")
//   - resourceUID: Unique identifier of the resource
//   - data: Condition change data (reason, message, etc.)
//
// Returns:
//   - *Event: CloudEvents-compliant condition change event
//   - error: If event creation fails or condition events are disabled
//
// Example:
//
//	event, err := NewConditionEvent("Ready", "True", "Device", "dev-123", conditionData)
//	// Generates event type: "io.fabrica.condition.ready"
func NewConditionEvent(conditionType, status, resourceKind, resourceUID string, data interface{}) (*Event, error) {
	if !AreConditionEventsEnabled() {
		return nil, fmt.Errorf("condition events are disabled")
	}

	config := GetEventConfig()

	// Build condition event type: conditionPrefix.conditiontype
	eventType := fmt.Sprintf("%s.%s",
		config.ConditionEventPrefix,
		strings.ToLower(conditionType))

	source := fmt.Sprintf("%s/resources/%s/%s/conditions/%s",
		config.Source, resourceKind, resourceUID, strings.ToLower(conditionType))

	event, err := NewEvent(eventType, source, data)
	if err != nil {
		return nil, err
	}

	// Add condition-specific extension attributes
	event.SetExtension("resourcekind", resourceKind)
	event.SetExtension("resourceuid", resourceUID)
	event.SetExtension("conditiontype", conditionType)
	event.SetExtension("conditionstatus", status)

	return event, nil
}

// ResourceKind returns the resource kind extension attribute
func (e *Event) ResourceKind() string {
	if val, ok := e.Extensions()["resourcekind"]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// ResourceUID returns the resource UID extension attribute
func (e *Event) ResourceUID() string {
	if val, ok := e.Extensions()["resourceuid"]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// EventHandler processes CloudEvents
type EventHandler func(ctx context.Context, event Event) error

// SubscriptionID uniquely identifies a subscription
type SubscriptionID string

// EventBus manages event publishing and subscription
type EventBus interface {
	// Publish a CloudEvent
	Publish(ctx context.Context, event Event) error

	// Subscribe to events by type pattern (supports wildcards)
	Subscribe(eventType string, handler EventHandler) (SubscriptionID, error)

	// Unsubscribe from events
	Unsubscribe(id SubscriptionID) error

	// Close the event bus
	Close() error
}

// GlobalEventBus holds the system-wide event bus instance
var globalEventBus EventBus
var busMutex sync.RWMutex

// SetGlobalEventBus configures the global event bus for the system
func SetGlobalEventBus(bus EventBus) {
	busMutex.Lock()
	defer busMutex.Unlock()
	globalEventBus = bus
}

// GetGlobalEventBus returns the current global event bus
func GetGlobalEventBus() EventBus {
	busMutex.RLock()
	defer busMutex.RUnlock()
	return globalEventBus
}

// PublishResourceEvent publishes a resource change event if events are enabled
//
// This is the main entry point for publishing resource events. It respects
// the global event configuration and only publishes if events are enabled.
//
// Parameters:
//   - ctx: Context for the publish operation
//   - action: The action that occurred (e.g., "created", "updated", "deleted")
//   - resourceKind: Kind of resource (e.g., "Device", "User")
//   - resourceUID: Unique identifier of the resource
//   - data: Event payload data
//
// Returns:
//   - error: If event creation or publishing fails, or if events are disabled
//
// Example:
//
//	err := PublishResourceEvent(ctx, "created", "Device", device.GetUID(), device)
func PublishResourceEvent(ctx context.Context, action, resourceKind, resourceUID string, data interface{}) error {
	// Check specific event type enablement first
	lifecycleActions := map[string]bool{
		"created": true, "create": true,
		"updated": true, "update": true,
		"deleted": true, "delete": true,
		"patched": true, "patch": true,
	}

	if lifecycleActions[strings.ToLower(action)] && !AreLifecycleEventsEnabled() {
		// Lifecycle events disabled - silently ignore
		return nil
	}

	if !IsEnabled() {
		// Events disabled - silently ignore
		return nil
	}

	bus := GetGlobalEventBus()
	if bus == nil {
		return fmt.Errorf("no event bus configured")
	}

	event, err := NewResourceEvent(action, resourceKind, resourceUID, data)
	if err != nil {
		return fmt.Errorf("failed to create resource event: %w", err)
	}

	return bus.Publish(ctx, *event)
}

// PublishConditionEvent publishes a condition change event if condition events are enabled
//
// This function is called automatically when conditions change on resources.
// It respects both the general event enable flag and the condition-specific flag.
//
// Parameters:
//   - ctx: Context for the publish operation
//   - conditionType: The type of condition (e.g., "Ready", "Healthy")
//   - status: The new condition status ("True", "False", "Unknown")
//   - resourceKind: Kind of resource (e.g., "Device", "User")
//   - resourceUID: Unique identifier of the resource
//   - data: Condition change data (reason, message, previous status, etc.)
//
// Returns:
//   - error: If event creation or publishing fails, or if condition events are disabled
//
// Example:
//
//	err := PublishConditionEvent(ctx, "Ready", "True", "Device", device.GetUID(), conditionData)
func PublishConditionEvent(ctx context.Context, conditionType, status, resourceKind, resourceUID string, data interface{}) error {
	if !AreConditionEventsEnabled() {
		// Condition events disabled - silently ignore
		return nil
	}

	bus := GetGlobalEventBus()
	if bus == nil {
		return fmt.Errorf("no event bus configured")
	}

	event, err := NewConditionEvent(conditionType, status, resourceKind, resourceUID, data)
	if err != nil {
		return fmt.Errorf("failed to create condition event: %w", err)
	}

	return bus.Publish(ctx, *event)
}

// ConditionChangeData represents the payload for condition change events
type ConditionChangeData struct {
	// ConditionType is the type of condition that changed
	ConditionType string `json:"conditionType"`

	// Status is the new status of the condition
	Status string `json:"status"`

	// PreviousStatus is the previous status (if known)
	PreviousStatus string `json:"previousStatus,omitempty"`

	// Reason is the machine-readable reason for the change
	Reason string `json:"reason,omitempty"`

	// Message is the human-readable message explaining the change
	Message string `json:"message,omitempty"`

	// TransitionTime is when the condition changed
	TransitionTime time.Time `json:"transitionTime"`

	// ResourceKind is the kind of resource (duplicated for convenience)
	ResourceKind string `json:"resourceKind"`

	// ResourceUID is the unique identifier of the resource
	ResourceUID string `json:"resourceUID"`
}

// ResourceChangeData represents the payload for general resource change events
type ResourceChangeData struct {
	// Action is the action that occurred
	Action string `json:"action"`

	// ResourceKind is the kind of resource
	ResourceKind string `json:"resourceKind"`

	// ResourceUID is the unique identifier of the resource
	ResourceUID string `json:"resourceUID"`

	// ResourceName is the human-readable name of the resource
	ResourceName string `json:"resourceName,omitempty"`

	// ChangeTime is when the change occurred
	ChangeTime time.Time `json:"changeTime"`

	// Metadata contains additional context about the change
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Resource contains the full resource data (optional, for create/update events)
	Resource interface{} `json:"resource,omitempty"`
}

// PublishResourceCreated publishes a "created" event for a resource
func PublishResourceCreated(ctx context.Context, resourceKind, resourceUID, resourceName string, resource interface{}) error {
	data := ResourceChangeData{
		Action:       "created",
		ResourceKind: resourceKind,
		ResourceUID:  resourceUID,
		ResourceName: resourceName,
		ChangeTime:   time.Now(),
		Resource:     resource,
	}

	return PublishResourceEvent(ctx, "created", resourceKind, resourceUID, data)
}

// PublishResourceUpdated publishes an "updated" event for a resource
func PublishResourceUpdated(ctx context.Context, resourceKind, resourceUID, resourceName string, resource interface{}, metadata map[string]interface{}) error {
	data := ResourceChangeData{
		Action:       "updated",
		ResourceKind: resourceKind,
		ResourceUID:  resourceUID,
		ResourceName: resourceName,
		ChangeTime:   time.Now(),
		Resource:     resource,
		Metadata:     metadata,
	}

	return PublishResourceEvent(ctx, "updated", resourceKind, resourceUID, data)
}

// PublishResourceDeleted publishes a "deleted" event for a resource
func PublishResourceDeleted(ctx context.Context, resourceKind, resourceUID, resourceName string, metadata map[string]interface{}) error {
	data := ResourceChangeData{
		Action:       "deleted",
		ResourceKind: resourceKind,
		ResourceUID:  resourceUID,
		ResourceName: resourceName,
		ChangeTime:   time.Now(),
		Metadata:     metadata,
	}

	return PublishResourceEvent(ctx, "deleted", resourceKind, resourceUID, data)
}

// PublishResourcePatched publishes a "patched" event for a resource (for partial updates)
func PublishResourcePatched(ctx context.Context, resourceKind, resourceUID, resourceName string, resource interface{}, patchData map[string]interface{}) error {
	metadata := map[string]interface{}{
		"patchData": patchData,
	}

	data := ResourceChangeData{
		Action:       "patched",
		ResourceKind: resourceKind,
		ResourceUID:  resourceUID,
		ResourceName: resourceName,
		ChangeTime:   time.Now(),
		Resource:     resource,
		Metadata:     metadata,
	}

	return PublishResourceEvent(ctx, "patched", resourceKind, resourceUID, data)
}

// generateEventID generates a unique event ID
func generateEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "evt-" + hex.EncodeToString(b)[:12]
}
