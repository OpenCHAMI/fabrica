// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package reconcile

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alexlovelltroy/fabrica/pkg/events"
	"github.com/alexlovelltroy/fabrica/pkg/storage"
)

// Controller manages the lifecycle of reconcilers.
//
// The controller:
//   - Registers reconcilers for different resource types
//   - Watches for resource change events
//   - Queues reconciliation requests
//   - Dispatches work to reconcilers
//   - Handles requeueing for periodic reconciliation
type Controller struct {
	reconcilers map[string]Reconciler
	queue       *WorkQueue
	eventBus    events.EventBus
	storage     storage.StorageBackend
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	logger      Logger
	workerCount int
}

// NewController creates a new reconciliation controller.
//
// Parameters:
//   - eventBus: Event bus for watching resource changes
//   - storage: Storage backend for loading resources
//
// Returns:
//   - *Controller: Initialized controller
func NewController(eventBus events.EventBus, storage storage.StorageBackend) *Controller {
	ctx, cancel := context.WithCancel(context.Background())

	return &Controller{
		reconcilers: make(map[string]Reconciler),
		queue:       NewWorkQueue(),
		eventBus:    eventBus,
		storage:     storage,
		ctx:         ctx,
		cancel:      cancel,
		logger:      NewDefaultLogger(),
		workerCount: 5, // Default worker count
	}
}

// RegisterReconciler registers a reconciler for a resource kind.
//
// Parameters:
//   - reconciler: Reconciler implementation for a specific resource type
//
// Returns:
//   - error: If reconciler for this kind is already registered
func (c *Controller) RegisterReconciler(reconciler Reconciler) error {
	kind := reconciler.GetResourceKind()

	if _, exists := c.reconcilers[kind]; exists {
		return fmt.Errorf("reconciler for kind %s already registered", kind)
	}

	c.reconcilers[kind] = reconciler
	c.logger.Infof("Registered reconciler for %s", kind)

	return nil
}

// Start begins the reconciliation controller.
//
// This:
//   - Starts worker goroutines
//   - Subscribes to resource change events
//   - Begins processing the work queue
//
// Parameters:
//   - ctx: Context for cancellation
//
// Returns:
//   - error: If startup fails
func (c *Controller) Start(_ context.Context) error {
	c.logger.Infof("Starting reconciliation controller with %d workers", c.workerCount)

	// Subscribe to all resource events (generic pattern - can be customized per application)
	_, err := c.eventBus.Subscribe("**", c.handleEvent)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Start worker goroutines
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}

	c.logger.Infof("Reconciliation controller started")

	return nil
}

// Stop gracefully shuts down the controller.
//
// This waits for all workers to finish processing their current items.
func (c *Controller) Stop() error {
	c.logger.Infof("Stopping reconciliation controller")

	c.cancel()
	c.queue.ShutDown()
	c.wg.Wait()

	c.logger.Infof("Reconciliation controller stopped")

	return nil
}

// Enqueue adds a reconciliation request to the work queue.
//
// Parameters:
//   - request: Reconciliation request
//
// Returns:
//   - error: If enqueueing fails
func (c *Controller) Enqueue(request ReconcileRequest) error {
	c.queue.Add(request)
	return nil
}

// EnqueueAfter adds a reconciliation request to be processed after a delay.
//
// Parameters:
//   - request: Reconciliation request
//   - delay: Duration to wait before processing
func (c *Controller) EnqueueAfter(request ReconcileRequest, delay time.Duration) {
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			c.queue.Add(request)
		case <-c.ctx.Done():
			return
		}
	}()
}

// worker processes items from the work queue.
func (c *Controller) worker(id int) {
	defer c.wg.Done()

	c.logger.Debugf("Worker %d started", id)

	for {
		item, shutdown := c.queue.Get()
		if shutdown {
			c.logger.Debugf("Worker %d shutting down", id)
			return
		}

		request, ok := item.(ReconcileRequest)
		if !ok {
			c.logger.Errorf("Worker %d: invalid item type in queue", id)
			c.queue.Done(item)
			continue
		}

		c.processRequest(request)
		c.queue.Done(item)
	}
}

// processRequest processes a single reconciliation request.
func (c *Controller) processRequest(request ReconcileRequest) {
	ctx := context.Background() // TODO: Add timeout/deadline

	c.logger.Debugf("Processing reconciliation for %s/%s (reason: %s)",
		request.ResourceKind, request.ResourceUID, request.Reason)

	// Get reconciler for this resource kind
	reconciler, exists := c.reconcilers[request.ResourceKind]
	if !exists {
		c.logger.Warnf("No reconciler registered for kind %s", request.ResourceKind)
		return
	}

	// Load resource from storage
	resource, err := c.loadResource(ctx, request.ResourceKind, request.ResourceUID)
	if err != nil {
		c.logger.Errorf("Failed to load resource %s/%s: %v",
			request.ResourceKind, request.ResourceUID, err)
		return
	}

	// Call reconciler
	result, err := reconciler.Reconcile(ctx, resource)
	if err != nil {
		c.logger.Errorf("Reconciliation failed for %s/%s: %v",
			request.ResourceKind, request.ResourceUID, err)

		// Requeue on error
		if result.Requeue || result.RequeueAfter > 0 {
			c.enqueueResult(request, result)
		} else {
			// Default retry after 30 seconds
			c.EnqueueAfter(request, 30*time.Second)
		}
		return
	}

	c.logger.Debugf("Reconciliation successful for %s/%s",
		request.ResourceKind, request.ResourceUID)

	// Handle requeueing based on result
	if result.Requeue || result.RequeueAfter > 0 {
		c.enqueueResult(request, result)
	}
}

// enqueueResult handles requeueing based on reconciliation result.
func (c *Controller) enqueueResult(request ReconcileRequest, result Result) {
	if result.Requeue {
		// Immediate requeue
		c.queue.Add(request)
	} else if result.RequeueAfter > 0 {
		// Delayed requeue
		c.EnqueueAfter(request, result.RequeueAfter)
	}
}

// loadResource loads a resource from storage.
func (c *Controller) loadResource(ctx context.Context, kind, uid string) (interface{}, error) {
	// Load raw resource data
	data, err := c.storage.Load(ctx, kind, uid)
	if err != nil {
		return nil, err
	}

	// TODO: Unmarshal to appropriate type based on kind
	// For now, return raw data
	return data, nil
}

// handleEvent processes resource change events.
func (c *Controller) handleEvent(_ context.Context, event events.Event) error {
	// Extract resource kind and UID from event
	resourceKind := event.ResourceKind()
	resourceUID := event.ResourceUID()

	if resourceKind == "" || resourceUID == "" {
		// Not a resource event, skip
		return nil
	}

	// Check if we have a reconciler for this kind
	if _, exists := c.reconcilers[resourceKind]; !exists {
		// No reconciler registered, skip
		return nil
	}

	// Determine reason from event type
	reason := fmt.Sprintf("Event: %s", event.Type())

	// Enqueue reconciliation request
	request := ReconcileRequest{
		ResourceKind: resourceKind,
		ResourceUID:  resourceUID,
		Reason:       reason,
	}

	return c.Enqueue(request)
}

// ReconcileRequest represents a request to reconcile a resource.
//
//nolint:revive // "ReconcileRequest" name is intentional; "Request" alone would be ambiguous
type ReconcileRequest struct {
	// ResourceKind is the kind of resource (e.g., "BMC", "Node")
	ResourceKind string

	// ResourceUID is the unique identifier of the resource
	ResourceUID string

	// Reason explains why this reconciliation was triggered
	Reason string
}

// String returns a string representation of the request.
func (r ReconcileRequest) String() string {
	return fmt.Sprintf("%s/%s", r.ResourceKind, r.ResourceUID)
}
