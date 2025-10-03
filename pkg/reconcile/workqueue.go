package reconcile

import (
	"sync"
	"time"
)

// WorkQueue manages a queue of work items with rate limiting and deduplication.
//
// Features:
//   - Thread-safe
//   - Automatic deduplication
//   - Rate limiting
//   - Delayed requeueing
//   - Graceful shutdown
type WorkQueue struct {
	queue        []interface{}
	processing   map[interface{}]struct{}
	mu           sync.RWMutex
	cond         *sync.Cond
	shuttingDown bool
}

// NewWorkQueue creates a new work queue.
func NewWorkQueue() *WorkQueue {
	wq := &WorkQueue{
		queue:      []interface{}{},
		processing: make(map[interface{}]struct{}),
	}
	wq.cond = sync.NewCond(&wq.mu)
	return wq
}

// Add adds an item to the queue.
//
// If the item is already in the queue or being processed, it won't be added again.
// This provides automatic deduplication.
//
// Parameters:
//   - item: Item to add to the queue
func (q *WorkQueue) Add(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.shuttingDown {
		return
	}

	// Check if already processing
	if _, exists := q.processing[item]; exists {
		return
	}

	// Check if already in queue
	for _, existing := range q.queue {
		if existing == item {
			return
		}
	}

	// Add to queue
	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// AddAfter adds an item to the queue after a delay.
//
// This is useful for requeueing items that should be processed later.
//
// Parameters:
//   - item: Item to add
//   - delay: Duration to wait before adding
func (q *WorkQueue) AddAfter(item interface{}, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		q.Add(item)
	}()
}

// Get retrieves an item from the queue.
//
// This blocks until an item is available or the queue is shut down.
//
// Returns:
//   - interface{}: The next item to process
//   - bool: false if the queue is shutting down, true otherwise
func (q *WorkQueue) Get() (interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.queue) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}

	if q.shuttingDown {
		return nil, false
	}

	// Get first item
	item := q.queue[0]
	q.queue = q.queue[1:]

	// Mark as processing
	q.processing[item] = struct{}{}

	return item, true
}

// Done marks an item as finished processing.
//
// This removes the item from the processing set, allowing it to be added again.
//
// Parameters:
//   - item: Item that finished processing
func (q *WorkQueue) Done(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, item)
}

// ShutDown initiates graceful shutdown of the queue.
//
// After shutdown:
//   - No new items can be added
//   - Get() will return false once the queue is empty
//   - Workers should stop after processing their current item
func (q *WorkQueue) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.shuttingDown = true
	q.cond.Broadcast()
}

// Len returns the number of items in the queue (excluding processing items).
func (q *WorkQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.queue)
}

// ProcessingCount returns the number of items currently being processed.
func (q *WorkQueue) ProcessingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.processing)
}

// RateLimitedWorkQueue extends WorkQueue with rate limiting.
//
// This prevents excessive reconciliation attempts for resources that are
// frequently updated or failing.
type RateLimitedWorkQueue struct {
	*WorkQueue
	limiter RateLimiter
}

// RateLimiter determines when an item can be requeued.
type RateLimiter interface {
	// When returns the duration to wait before requeueing
	When(item interface{}) time.Duration

	// Forget resets the rate limit for an item
	Forget(item interface{})

	// NumRequeues returns the number of times an item has been requeued
	NumRequeues(item interface{}) int
}

// ExponentialBackoffRateLimiter implements exponential backoff.
type ExponentialBackoffRateLimiter struct {
	failures  map[interface{}]int
	baseDelay time.Duration
	maxDelay  time.Duration
	mu        sync.RWMutex
}

// NewExponentialBackoffRateLimiter creates a new exponential backoff rate limiter.
//
// Parameters:
//   - baseDelay: Initial delay (e.g., 1 second)
//   - maxDelay: Maximum delay (e.g., 5 minutes)
func NewExponentialBackoffRateLimiter(baseDelay, maxDelay time.Duration) *ExponentialBackoffRateLimiter {
	return &ExponentialBackoffRateLimiter{
		failures:  make(map[interface{}]int),
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
	}
}

// When returns the delay before requeueing based on exponential backoff.
func (r *ExponentialBackoffRateLimiter) When(item interface{}) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	failures := r.failures[item]
	r.failures[item] = failures + 1

	// Calculate exponential backoff: baseDelay * 2^failures
	delay := r.baseDelay
	for i := 0; i < failures; i++ {
		delay *= 2
		if delay > r.maxDelay {
			delay = r.maxDelay
			break
		}
	}

	return delay
}

// Forget resets the failure count for an item.
func (r *ExponentialBackoffRateLimiter) Forget(item interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failures, item)
}

// NumRequeues returns the number of times an item has been requeued.
func (r *ExponentialBackoffRateLimiter) NumRequeues(item interface{}) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failures[item]
}

// NewRateLimitedWorkQueue creates a new rate-limited work queue.
func NewRateLimitedWorkQueue(limiter RateLimiter) *RateLimitedWorkQueue {
	return &RateLimitedWorkQueue{
		WorkQueue: NewWorkQueue(),
		limiter:   limiter,
	}
}

// AddRateLimited adds an item to the queue with rate limiting.
func (q *RateLimitedWorkQueue) AddRateLimited(item interface{}) {
	delay := q.limiter.When(item)
	if delay > 0 {
		q.AddAfter(item, delay)
	} else {
		q.Add(item)
	}
}

// Forget resets the rate limit for an item.
func (q *RateLimitedWorkQueue) Forget(item interface{}) {
	q.limiter.Forget(item)
}
