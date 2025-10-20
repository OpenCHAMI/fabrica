// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package reconcile

import (
	"sync"
	"testing"
	"time"
)

func TestWorkQueue_AddAndGet(t *testing.T) {
	q := NewWorkQueue()

	// Add an item
	item := "test-item"
	q.Add(item)

	// Get should return the item
	got, ok := q.Get()
	if !ok {
		t.Fatal("Get() returned ok=false, expected true")
	}
	if got != item {
		t.Errorf("Get() = %v, want %v", got, item)
	}
}

func TestWorkQueue_GetBlocksUntilItemAvailable(t *testing.T) {
	q := NewWorkQueue()

	done := make(chan bool)
	var got interface{}
	var ok bool

	// Start goroutine that will block on Get()
	go func() {
		got, ok = q.Get()
		done <- true
	}()

	// Give it a moment to start waiting
	time.Sleep(50 * time.Millisecond)

	// Add an item
	item := "test-item"
	q.Add(item)

	// Wait for Get() to return
	select {
	case <-done:
		if !ok {
			t.Fatal("Get() returned ok=false, expected true")
		}
		if got != item {
			t.Errorf("Get() = %v, want %v", got, item)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Get() did not return within timeout")
	}
}

func TestWorkQueue_ShutdownReturnsNilAndFalse(t *testing.T) {
	q := NewWorkQueue()

	// Shutdown the queue
	q.ShutDown()

	// Get should return nil, false
	got, ok := q.Get()
	if ok {
		t.Error("Get() returned ok=true after shutdown, expected false")
	}
	if got != nil {
		t.Errorf("Get() = %v after shutdown, want nil", got)
	}
}

func TestWorkQueue_ShutdownUnblocksWaitingWorkers(t *testing.T) {
	q := NewWorkQueue()

	done := make(chan bool)
	var ok bool

	// Start goroutine that will block on Get()
	go func() {
		_, ok = q.Get()
		done <- true
	}()

	// Give it a moment to start waiting
	time.Sleep(50 * time.Millisecond)

	// Shutdown should unblock the waiting Get()
	q.ShutDown()

	// Wait for Get() to return
	select {
	case <-done:
		if ok {
			t.Error("Get() returned ok=true after shutdown, expected false")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Get() did not return within timeout after shutdown")
	}
}

func TestWorkQueue_DeduplicationSameItem(t *testing.T) {
	q := NewWorkQueue()

	item := "test-item"
	q.Add(item)
	q.Add(item) // Duplicate

	// Should only get one item
	got, ok := q.Get()
	if !ok {
		t.Fatal("Get() returned ok=false, expected true")
	}
	if got != item {
		t.Errorf("Get() = %v, want %v", got, item)
	}

	// Mark as done
	q.Done(item)

	// Queue should be empty now
	if q.Len() != 0 {
		t.Errorf("Queue length = %d after deduplication, want 0", q.Len())
	}
}

func TestWorkQueue_DeduplicationWhileProcessing(t *testing.T) {
	q := NewWorkQueue()

	item := "test-item"
	q.Add(item)

	// Get the item (now it's processing)
	got, ok := q.Get()
	if !ok {
		t.Fatal("Get() returned ok=false, expected true")
	}

	// Try to add the same item while it's processing
	q.Add(item)

	// Queue should still be empty (item is in processing set)
	if q.Len() != 0 {
		t.Errorf("Queue length = %d while item is processing, want 0", q.Len())
	}

	// Mark as done
	q.Done(got)

	// Now we can add it again
	q.Add(item)
	if q.Len() != 1 {
		t.Errorf("Queue length = %d after Done(), want 1", q.Len())
	}
}

func TestWorkQueue_MultipleWorkers(t *testing.T) {
	q := NewWorkQueue()
	const numWorkers = 5
	const numItems = 100

	// Add items
	for i := 0; i < numItems; i++ {
		q.Add(i)
	}

	// Process with multiple workers
	var wg sync.WaitGroup
	processed := make(map[int]bool)
	var mu sync.Mutex

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				item, ok := q.Get()
				if !ok {
					return
				}
				num := item.(int)
				mu.Lock()
				if processed[num] {
					t.Errorf("Item %d processed twice", num)
				}
				processed[num] = true
				mu.Unlock()
				q.Done(item)
			}
		}()
	}

	// Give workers time to process
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	q.ShutDown()
	wg.Wait()

	// Verify all items were processed
	if len(processed) != numItems {
		t.Errorf("Processed %d items, want %d", len(processed), numItems)
	}
}

func TestWorkQueue_Len(t *testing.T) {
	q := NewWorkQueue()

	if q.Len() != 0 {
		t.Errorf("Len() = %d for empty queue, want 0", q.Len())
	}

	q.Add("item1")
	if q.Len() != 1 {
		t.Errorf("Len() = %d after adding 1 item, want 1", q.Len())
	}

	q.Add("item2")
	q.Add("item3")
	if q.Len() != 3 {
		t.Errorf("Len() = %d after adding 3 items, want 3", q.Len())
	}

	// Get an item (moves to processing)
	item, _ := q.Get()
	if q.Len() != 2 {
		t.Errorf("Len() = %d after Get(), want 2", q.Len())
	}

	q.Done(item)
	if q.Len() != 2 {
		t.Errorf("Len() = %d after Done(), want 2", q.Len())
	}
}

func TestWorkQueue_ProcessingCount(t *testing.T) {
	q := NewWorkQueue()

	if q.ProcessingCount() != 0 {
		t.Errorf("ProcessingCount() = %d for empty queue, want 0", q.ProcessingCount())
	}

	q.Add("item1")
	q.Add("item2")

	// Get an item (moves to processing)
	item1, _ := q.Get()
	if q.ProcessingCount() != 1 {
		t.Errorf("ProcessingCount() = %d after first Get(), want 1", q.ProcessingCount())
	}

	// Get another item
	item2, _ := q.Get()
	if q.ProcessingCount() != 2 {
		t.Errorf("ProcessingCount() = %d after second Get(), want 2", q.ProcessingCount())
	}

	// Mark first as done
	q.Done(item1)
	if q.ProcessingCount() != 1 {
		t.Errorf("ProcessingCount() = %d after first Done(), want 1", q.ProcessingCount())
	}

	// Mark second as done
	q.Done(item2)
	if q.ProcessingCount() != 0 {
		t.Errorf("ProcessingCount() = %d after all Done(), want 0", q.ProcessingCount())
	}
}

func TestWorkQueue_AddAfter(t *testing.T) {
	q := NewWorkQueue()

	item := "delayed-item"
	delay := 100 * time.Millisecond

	start := time.Now()
	q.AddAfter(item, delay)

	// Queue should be empty initially
	if q.Len() != 0 {
		t.Errorf("Queue length = %d immediately after AddAfter, want 0", q.Len())
	}

	// Wait for item to be added
	time.Sleep(delay + 50*time.Millisecond)

	elapsed := time.Since(start)
	if elapsed < delay {
		t.Errorf("Item appeared after %v, want at least %v", elapsed, delay)
	}

	if q.Len() != 1 {
		t.Errorf("Queue length = %d after delay, want 1", q.Len())
	}
}
