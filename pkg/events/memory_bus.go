// Package events provides event bus implementations.
package events

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// InMemoryEventBus implements EventBus with in-memory channels.
//
// This implementation is suitable for:
//   - Development and testing
//   - Single-instance deployments
//   - Scenarios where event persistence is not required
//
// Characteristics:
//   - Low latency (microseconds)
//   - No durability (events lost on restart)
//   - Thread-safe
//   - Support for wildcard subscriptions
type InMemoryEventBus struct {
	subscribers map[string][]subscription
	eventQueue  chan Event
	bufferSize  int
	workerCount int
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	nextSubID   int
	subIDMu     sync.Mutex
}

// subscription represents an event subscription
type subscription struct {
	id      SubscriptionID
	pattern string
	handler EventHandler
}

// NewInMemoryEventBus creates a new in-memory event bus
//
// Parameters:
//   - bufferSize: Size of the event queue buffer (default: 1000)
//   - workerCount: Number of worker goroutines (default: 10)
//
// Returns:
//   - *InMemoryEventBus: Initialized event bus (must call Start())
func NewInMemoryEventBus(bufferSize, workerCount int) *InMemoryEventBus {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	if workerCount <= 0 {
		workerCount = 10
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &InMemoryEventBus{
		subscribers: make(map[string][]subscription),
		eventQueue:  make(chan Event, bufferSize),
		bufferSize:  bufferSize,
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
		nextSubID:   1,
	}
}

// Start begins processing events
//
// This must be called before publishing events.
// It starts worker goroutines that process events from the queue.
func (b *InMemoryEventBus) Start() {
	for i := 0; i < b.workerCount; i++ {
		b.wg.Add(1)
		go b.worker()
	}
}

// worker processes events from the queue
func (b *InMemoryEventBus) worker() {
	defer b.wg.Done()

	for {
		select {
		case <-b.ctx.Done():
			return
		case event := <-b.eventQueue:
			b.dispatch(event)
		}
	}
}

// dispatch sends an event to all matching subscribers
func (b *InMemoryEventBus) dispatch(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	eventType := event.Type()

	// Find all subscriptions that match this event type
	for _, subs := range b.subscribers {
		for _, sub := range subs {
			if matchesPattern(eventType, sub.pattern) {
				// Call handler in a goroutine to avoid blocking
				go func(h EventHandler) {
					// Create a new context for this handler
					ctx := context.Background()
					if err := h(ctx, event); err != nil {
						// Log error but don't stop processing
						// In production, this should use a proper logger
						fmt.Printf("Error handling event %s: %v\n", event.ID(), err)
					}
				}(sub.handler)
			}
		}
	}
}

// Publish publishes an event to the bus
//
// The event is queued and processed asynchronously by worker goroutines.
//
// Parameters:
//   - ctx: Context for cancellation
//   - event: CloudEvents-compliant event to publish
//
// Returns:
//   - error: If the event queue is full or the bus is closed
func (b *InMemoryEventBus) Publish(ctx context.Context, event Event) error {
	select {
	case <-b.ctx.Done():
		return fmt.Errorf("event bus is closed")
	case <-ctx.Done():
		return ctx.Err()
	case b.eventQueue <- event:
		return nil
	default:
		return fmt.Errorf("event queue is full")
	}
}

// Subscribe subscribes to events matching a pattern
//
// Pattern Syntax:
//   - Exact match: "io.example.resource.created"
//   - Single wildcard: "io.example.resource.*" (matches one segment)
//   - Multi wildcard: "io.example.**" (matches any remaining segments)
//
// Parameters:
//   - eventType: Pattern to match against event types
//   - handler: Function to call when matching events occur
//
// Returns:
//   - SubscriptionID: Unique ID for this subscription (use to unsubscribe)
//   - error: If subscription fails
//
// Example:
//
//	// Subscribe to all device events
//	id, err := bus.Subscribe("io.example.device.*", func(ctx context.Context, event Event) error {
//	    fmt.Printf("Device event: %s\n", event.Type())
//	    return nil
//	})
func (b *InMemoryEventBus) Subscribe(eventType string, handler EventHandler) (SubscriptionID, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Generate unique subscription ID
	b.subIDMu.Lock()
	id := SubscriptionID(fmt.Sprintf("sub-%d", b.nextSubID))
	b.nextSubID++
	b.subIDMu.Unlock()

	// Create subscription
	sub := subscription{
		id:      id,
		pattern: eventType,
		handler: handler,
	}

	// Add to subscribers map
	if b.subscribers[eventType] == nil {
		b.subscribers[eventType] = []subscription{}
	}
	b.subscribers[eventType] = append(b.subscribers[eventType], sub)

	return id, nil
}

// Unsubscribe removes a subscription
//
// Parameters:
//   - id: Subscription ID returned by Subscribe()
//
// Returns:
//   - error: If subscription not found
func (b *InMemoryEventBus) Unsubscribe(id SubscriptionID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Find and remove subscription
	for pattern, subs := range b.subscribers {
		for i, sub := range subs {
			if sub.id == id {
				// Remove subscription from slice
				b.subscribers[pattern] = append(subs[:i], subs[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("subscription not found: %s", id)
}

// Close shuts down the event bus
//
// This stops all workers and waits for them to finish processing.
// After Close() is called, no more events can be published.
func (b *InMemoryEventBus) Close() error {
	b.cancel()
	b.wg.Wait()
	close(b.eventQueue)
	return nil
}

// matchesPattern checks if an event type matches a subscription pattern
//
// Pattern matching rules:
//   - "*" matches exactly one segment
//   - "**" matches one or more segments
//   - Segments are separated by "."
//
// Examples:
//   - "io.example.device.created" matches "io.example.device.created"
//   - "io.example.device.created" matches "io.example.device.*"
//   - "io.example.device.created" matches "io.example.**"
//   - "io.example.device.created" does NOT match "io.example.user.*"
func matchesPattern(eventType, pattern string) bool {
	// Exact match
	if eventType == pattern {
		return true
	}

	eventParts := strings.Split(eventType, ".")
	patternParts := strings.Split(pattern, ".")

	// Check for multi-segment wildcard (**)
	for i, p := range patternParts {
		if p == "**" {
			// Match everything from this point
			return true
		}

		// Check if we've exhausted event parts
		if i >= len(eventParts) {
			return false
		}

		// Check for single-segment wildcard (*)
		if p == "*" {
			continue
		}

		// Exact segment match required
		if p != eventParts[i] {
			return false
		}
	}

	// All pattern parts matched, check if event has extra parts
	return len(eventParts) == len(patternParts)
}
