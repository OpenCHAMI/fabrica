// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package reconcile provides the reconciliation framework for inventory resources.
//
// The reconciliation system enables declarative infrastructure management by
// automatically reconciling the desired state (Spec) with the observed state (Status).
//
// Architecture:
//   - Reconciler: Interface for resource-specific reconciliation logic
//   - Controller: Manages reconciler lifecycle and work queue
//   - BaseReconciler: Common functionality for reconcilers
//   - WorkQueue: Queue for reconciliation requests
//
// Usage:
//
//	// Create a reconciler
//	type DeviceReconciler struct {
//	    reconcile.BaseReconciler
//	}
//
//	func (r *DeviceReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
//	    device := resource.(*Device)
//	    // Reconciliation logic here
//	    return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
//	}
//
//	// Register and start
//	controller := reconcile.NewController(eventBus, storage)
//	controller.RegisterReconciler(deviceReconciler)
//	controller.Start(ctx)
package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openchami/fabrica/pkg/events"
)

// Reconciler handles resource reconciliation.
//
// Implementations should:
//   - Be idempotent (safe to call multiple times)
//   - Update Status to reflect observed state
//   - Emit events for significant state changes
//   - Return appropriate Result for requeueing
type Reconciler interface {
	// Reconcile brings the resource to its desired state
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - resource: Resource to reconcile (type-specific)
	//
	// Returns:
	//   - Result: Indicates if/when to requeue
	//   - error: Any error that occurred
	Reconcile(ctx context.Context, resource interface{}) (Result, error)

	// GetResourceKind returns the resource kind this reconciler handles
	GetResourceKind() string
}

// Result indicates the outcome of reconciliation.
//
// The controller uses this to determine whether to requeue the resource
// for another reconciliation attempt.
type Result struct {
	// Requeue indicates if the resource should be requeued immediately
	Requeue bool

	// RequeueAfter indicates when to requeue (if > 0)
	// If both Requeue and RequeueAfter are set, Requeue takes precedence
	RequeueAfter time.Duration
}

// ClientInterface provides access to resource storage.
//
// This interface abstracts storage operations to make reconcilers testable.
type ClientInterface interface {
	// Get retrieves a resource by UID
	Get(ctx context.Context, kind, uid string) (interface{}, error)

	// List retrieves all resources of a kind
	List(ctx context.Context, kind string) ([]interface{}, error)

	// Update updates a resource
	Update(ctx context.Context, resource interface{}) error

	// Create creates a new resource
	Create(ctx context.Context, resource interface{}) error

	// Delete deletes a resource
	Delete(ctx context.Context, kind, uid string) error
}

// BaseReconciler provides common functionality for reconcilers.
//
// Resource-specific reconcilers should embed this struct to get:
//   - Event emission
//   - Status updates
//   - Condition management
//   - Logging helpers
type BaseReconciler struct {
	// Client provides access to resource storage
	Client ClientInterface

	// EventBus for publishing events
	EventBus events.EventBus

	// Logger for structured logging (optional)
	Logger Logger
}

// UpdateStatus updates the status of a resource in storage.
//
// IMPORTANT: This method loads a fresh copy of the resource from storage
// and only updates the status portion, preserving any spec changes that may
// have occurred concurrently. This prevents reconcilers from accidentally
// overwriting spec updates made by users.
//
// This is the recommended way for reconcilers to update status.
//
// Parameters:
//   - ctx: Context for cancellation
//   - resource: Resource with updated status
//
// Returns:
//   - error: If update fails
func (r *BaseReconciler) UpdateStatus(ctx context.Context, resource interface{}) error {
	if r.Client == nil {
		return fmt.Errorf("client is not configured")
	}

	// Extract resource kind and UID
	type resourceMetadata interface {
		GetKind() string
		GetUID() string
	}

	res, ok := resource.(resourceMetadata)
	if !ok {
		return fmt.Errorf("resource does not implement required metadata methods")
	}

	kind := res.GetKind()
	uid := res.GetUID()

	// Load fresh copy from storage to avoid overwriting spec changes
	current, err := r.Client.Get(ctx, kind, uid)
	if err != nil {
		return fmt.Errorf("failed to load current resource: %w", err)
	}

	// Marshal both resources to JSON for status extraction
	resourceData, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	currentData, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("failed to marshal current resource: %w", err)
	}

	var resourceMap map[string]interface{}
	if err := json.Unmarshal(resourceData, &resourceMap); err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	var currentMap map[string]interface{}
	if err := json.Unmarshal(currentData, &currentMap); err != nil {
		return fmt.Errorf("failed to unmarshal current resource: %w", err)
	}

	// Copy status from reconciled resource to current resource
	if status, ok := resourceMap["status"]; ok {
		currentMap["status"] = status
	}

	// Update metadata.updatedAt timestamp
	if metadata, ok := currentMap["metadata"].(map[string]interface{}); ok {
		metadata["updatedAt"] = time.Now().Format(time.RFC3339)
		currentMap["metadata"] = metadata
	}

	// Marshal back to current resource
	updatedData, err := json.Marshal(currentMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated resource: %w", err)
	}

	if err := json.Unmarshal(updatedData, current); err != nil {
		return fmt.Errorf("failed to unmarshal updated resource: %w", err)
	}

	// Save the updated resource (with preserved spec, updated status)
	return r.Client.Update(ctx, current)
}

// EmitEvent publishes an event for a resource.
//
// This is a convenience method that creates and publishes a CloudEvents-compliant event.
//
// Parameters:
//   - ctx: Context for cancellation
//   - eventType: CloudEvents type (e.g., "io.example.device.connected")
//   - resource: Resource that triggered the event
//
// Returns:
//   - error: If event emission fails
func (r *BaseReconciler) EmitEvent(ctx context.Context, eventType string, resource interface{}) error {
	if r.EventBus == nil {
		return fmt.Errorf("event bus is not configured")
	}

	// Extract resource kind and UID
	// Resources should have GetKind() and GetUID() methods
	var kind, uid string

	// Use type assertion to get metadata
	type resourceMetadata interface {
		GetKind() string
		GetUID() string
	}

	if res, ok := resource.(resourceMetadata); ok {
		kind = res.GetKind()
		uid = res.GetUID()
	} else {
		// Try reflection as fallback
		return fmt.Errorf("resource does not implement required metadata methods")
	}

	// Create event
	event, err := events.NewResourceEvent(
		eventType,
		kind,
		uid,
		resource,
	)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	// Publish event
	if err := r.EventBus.Publish(ctx, *event); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	if r.Logger != nil {
		r.Logger.Infof("Emitted event %s for %s/%s", eventType, kind, uid)
	}

	return nil
}

// SetCondition sets a condition on a resource.
//
// This is a helper for managing resource conditions in the Kubernetes style.
//
// Parameters:
//   - resource: Resource to update (must have Status.Conditions)
//   - condType: Condition type (e.g., "Ready", "Healthy")
//   - status: Condition status ("True", "False", "Unknown")
//   - reason: Machine-readable reason code
//   - message: Human-readable message
//
// Returns:
//   - error: If resource doesn't support conditions
func (r *BaseReconciler) SetCondition(resource interface{}, condType, status, reason, message string) error {
	// This is a simplified version - in practice, you'd use reflection or
	// interface assertions to set conditions on resources that support them

	// For now, we'll just marshal and unmarshal to check if conditions exist
	data, err := json.Marshal(resource)
	if err != nil {
		return err
	}

	var temp map[string]interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Check if resource has status.conditions
	statusMap, ok := temp["status"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("resource does not have status field")
	}

	conditions, ok := statusMap["conditions"].([]interface{})
	if !ok {
		// Initialize conditions if not present
		conditions = []interface{}{}
	}

	// Create new condition
	newCondition := map[string]interface{}{
		"type":               condType,
		"status":             status,
		"reason":             reason,
		"message":            message,
		"lastTransitionTime": time.Now().Format(time.RFC3339),
	}

	// Update or append condition
	found := false
	for i, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		if condMap["type"] == condType {
			conditions[i] = newCondition
			found = true
			break
		}
	}

	if !found {
		conditions = append(conditions, newCondition)
	}

	statusMap["conditions"] = conditions
	temp["status"] = statusMap

	// Marshal back to resource
	updatedData, err := json.Marshal(temp)
	if err != nil {
		return err
	}

	return json.Unmarshal(updatedData, resource)
}

// Logger interface for structured logging.
//
// Implementations should provide structured logging with levels.
type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

// defaultLogger is a simple logger that writes to stdout
type defaultLogger struct{}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

// NewDefaultLogger creates a simple stdout logger
func NewDefaultLogger() Logger {
	return &defaultLogger{}
}
