// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package reconcile

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alexlovelltroy/fabrica/pkg/events"
	"github.com/alexlovelltroy/fabrica/pkg/storage"
)

// Mock reconciler for testing
type mockReconciler struct {
	BaseReconciler
	callCount   int
	mu          sync.Mutex
	shouldError bool
	result      Result
	kind        string
}

func (m *mockReconciler) Reconcile(ctx context.Context, resource interface{}) (Result, error) { //nolint:revive
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.shouldError {
		return Result{}, context.DeadlineExceeded
	}
	return m.result, nil
}

func (m *mockReconciler) GetResourceKind() string {
	if m.kind != "" {
		return m.kind
	}
	return "TestResource"
}

func (m *mockReconciler) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Test that worker correctly handles the boolean from queue.Get()
func TestController_WorkerShutdownLogic(t *testing.T) {
	// This test specifically catches the regression we fixed where
	// the boolean from queue.Get() was inverted
	q := NewWorkQueue()

	// Simulate what happens when queue is shutdown
	q.ShutDown()

	// Worker should recognize shutdown by checking !ok
	item, ok := q.Get()

	// When queue is shutdown, Get() should return (nil, false)
	if ok {
		t.Error("queue.Get() returned ok=true after shutdown, expected false")
	}
	if item != nil {
		t.Errorf("queue.Get() returned item=%v after shutdown, expected nil", item)
	}

	// The worker code should check: if !ok { return }
	// NOT: if ok { return }
	if !ok {
		// This is correct - worker should exit when !ok
		t.Log("Worker would correctly exit on shutdown")
	} else {
		t.Error("Worker would NOT exit on shutdown - regression detected!")
	}
}

func TestController_EnqueueAndProcess(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for storage
	tempDir := t.TempDir()

	// Create event bus and storage
	eventBus := events.NewInMemoryEventBus(100, 1)
	eventBus.Start()

	fileStorage, err := storage.NewFileBackend(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create test resource
	testResource := map[string]interface{}{
		"kind": "TestResource",
		"metadata": map[string]interface{}{
			"uid":  "test-123",
			"name": "test-resource",
		},
	}
	resourceData, _ := json.Marshal(testResource)
	err = fileStorage.Save(ctx, "TestResource", "test-123", resourceData)
	if err != nil {
		t.Fatalf("Failed to save test resource: %v", err)
	}

	// Create controller
	controller := NewController(eventBus, fileStorage)

	// Create and register mock reconciler
	reconciler := &mockReconciler{
		BaseReconciler: BaseReconciler{
			Logger: NewDefaultLogger(),
		},
		result: Result{},
	}
	err = controller.RegisterReconciler(reconciler)
	if err != nil {
		t.Fatalf("Failed to register reconciler: %v", err)
	}

	// Start controller
	err = controller.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}
	defer controller.Stop() //nolint:errcheck

	// Enqueue a reconciliation request
	request := ReconcileRequest{
		ResourceKind: "TestResource",
		ResourceUID:  "test-123",
		Reason:       "Test",
	}
	err = controller.Enqueue(request)
	if err != nil {
		t.Fatalf("Failed to enqueue request: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify reconciler was called
	if reconciler.GetCallCount() != 1 {
		t.Errorf("Reconciler call count = %d, want 1", reconciler.GetCallCount())
	}
}

func TestController_EventTriggersReconciliation(t *testing.T) {
	ctx := context.Background()

	// Initialize event system for testing
	config := &events.EventConfig{
		Enabled:                true,
		EventTypePrefix:        "io.fabrica",
		LifecycleEventsEnabled: true,
		ConditionEventsEnabled: true,
	}
	events.SetEventConfig(config)

	// Create temp directory for storage
	tempDir := t.TempDir()

	// Create event bus and storage
	eventBus := events.NewInMemoryEventBus(100, 1)
	eventBus.Start()

	fileStorage, err := storage.NewFileBackend(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create test resource
	testResource := map[string]interface{}{
		"kind": "TestResource",
		"metadata": map[string]interface{}{
			"uid":  "test-456",
			"name": "test-resource",
		},
	}
	resourceData, _ := json.Marshal(testResource)
	err = fileStorage.Save(ctx, "TestResource", "test-456", resourceData)
	if err != nil {
		t.Fatalf("Failed to save test resource: %v", err)
	}

	// Create controller
	controller := NewController(eventBus, fileStorage)

	// Create and register mock reconciler
	reconciler := &mockReconciler{
		BaseReconciler: BaseReconciler{
			Logger: NewDefaultLogger(),
		},
		result: Result{},
	}
	err = controller.RegisterReconciler(reconciler)
	if err != nil {
		t.Fatalf("Failed to register reconciler: %v", err)
	}

	// Start controller
	err = controller.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}
	defer controller.Stop() //nolint:errcheck

	// Publish resource event
	event, err := events.NewResourceEvent(
		"io.fabrica.testresource.created",
		"TestResource",
		"test-456",
		testResource,
	)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	err = eventBus.Publish(ctx, *event)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify reconciler was called
	if reconciler.GetCallCount() != 1 {
		t.Errorf("Reconciler call count = %d, want 1", reconciler.GetCallCount())
	}
}

func TestController_OnlyReconcileRegisteredKinds(t *testing.T) {
	ctx := context.Background()

	// Configure events system
	eventConfig := &events.EventConfig{
		Enabled:                true,
		LifecycleEventsEnabled: true,
		ConditionEventsEnabled: true,
		EventTypePrefix:        "io.fabrica",
		Source:                 "fabrica-test",
	}
	events.SetEventConfig(eventConfig)

	// Create temp directory for storage
	tempDir := t.TempDir()

	// Create event bus and storage
	eventBus := events.NewInMemoryEventBus(100, 1)
	eventBus.Start()

	fileStorage, err := storage.NewFileBackend(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create test resources
	registeredResource := map[string]interface{}{
		"kind": "RegisteredResource",
		"metadata": map[string]interface{}{
			"uid":  "reg-123",
			"name": "registered",
		},
	}
	unregisteredResource := map[string]interface{}{
		"kind": "UnregisteredResource",
		"metadata": map[string]interface{}{
			"uid":  "unreg-123",
			"name": "unregistered",
		},
	}

	regData, _ := json.Marshal(registeredResource)
	unregData, _ := json.Marshal(unregisteredResource)
	err = fileStorage.Save(ctx, "RegisteredResource", "reg-123", regData)
	if err != nil {
		t.Fatalf("Failed to save registered resource: %v", err)
	}
	err = fileStorage.Save(ctx, "UnregisteredResource", "unreg-123", unregData)
	if err != nil {
		t.Fatalf("Failed to save unregistered resource: %v", err)
	}

	// Create controller
	controller := NewController(eventBus, fileStorage)

	// Create and register reconciler ONLY for RegisteredResource
	reconciler := &mockReconciler{
		BaseReconciler: BaseReconciler{
			Logger: NewDefaultLogger(),
		},
		result: Result{},
		kind:   "RegisteredResource",
	}

	err = controller.RegisterReconciler(reconciler)
	if err != nil {
		t.Fatalf("Failed to register reconciler: %v", err)
	}

	// Start controller
	err = controller.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}
	defer controller.Stop() //nolint:errcheck

	// Publish event for registered resource
	event1, _ := events.NewResourceEvent(
		"io.fabrica.registeredresource.created",
		"RegisteredResource",
		"reg-123",
		registeredResource,
	)
	err = eventBus.Publish(ctx, *event1)
	if err != nil {
		t.Fatalf("Failed to publish registered resource event: %v", err)
	}

	// Publish event for unregistered resource
	event2, _ := events.NewResourceEvent(
		"io.fabrica.unregisteredresource.created",
		"UnregisteredResource",
		"unreg-123",
		unregisteredResource,
	)
	err = eventBus.Publish(ctx, *event2)
	if err != nil {
		t.Fatalf("Failed to publish unregistered resource event: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify reconciler was called only for registered resource
	if reconciler.GetCallCount() != 1 {
		t.Errorf("Reconciler call count = %d, want 1 (only for registered resource)", reconciler.GetCallCount())
	}
}

func TestController_WorkerCountRespected(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for storage
	tempDir := t.TempDir()

	eventBus := events.NewInMemoryEventBus(100, 1)
	eventBus.Start()

	fileStorage, err := storage.NewFileBackend(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	controller := NewController(eventBus, fileStorage)

	// Verify default worker count
	if controller.workerCount != 5 {
		t.Errorf("Default worker count = %d, want 5", controller.workerCount)
	}

	// Start controller
	err = controller.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Stop controller
	err = controller.Stop()
	if err != nil {
		t.Fatalf("Failed to stop controller: %v", err)
	}

	// The test passes if we get here without hanging
	// (all workers should have stopped)
}

func TestController_GracefulShutdown(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for storage
	tempDir := t.TempDir()

	eventBus := events.NewInMemoryEventBus(100, 1)
	eventBus.Start()

	fileStorage, err := storage.NewFileBackend(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	controller := NewController(eventBus, fileStorage)

	// Start controller
	err = controller.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}

	// Enqueue some items
	for i := 0; i < 10; i++ {
		err = controller.Enqueue(ReconcileRequest{
			ResourceKind: "Test",
			ResourceUID:  "test",
			Reason:       "test",
		})
		if err != nil {
			t.Fatalf("Failed to enqueue item: %v", err)
		}
	}

	// Stop should wait for workers to finish
	done := make(chan bool)
	go func() {
		controller.Stop() //nolint:errcheck
		done <- true
	}()

	// Stop should complete within reasonable time
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Controller.Stop() did not complete within timeout")
	}
}
