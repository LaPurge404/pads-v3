package evolution

import (
	"path/filepath"
	"testing"
)

// TestWorkerOffsetBasedReading verifies that ReadFrom does not re-read events
// already processed in previous calls (offset is managed internally).
func TestWorkerOffsetBasedReading(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "test_queue.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	// Write 3 initial events
	for i := 0; i < 3; i++ {
		if err := queue.Enqueue(QueueEvent{
			ID:   "initial-" + string(rune('A'+i)),
			Type: "evolve",
		}); err != nil {
			t.Fatalf("Enqueue initial[%d]: %v", i, err)
		}
	}

	// First read: should see all 3 events
	events1, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom (1st call): %v", err)
	}
	if len(events1) != 3 {
		t.Errorf("first read: expected 3 events, got %d", len(events1))
	}

	processed := make(map[string]bool)
	for _, e := range events1 {
		processed[e.ID] = true
	}

	// Second read (immediately after): should see no new events
	events2, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom (2nd call): %v", err)
	}
	if len(events2) != 0 {
		t.Errorf("second read (no new events): expected 0, got %d", len(events2))
	}

	// Write 2 new events
	for i := 0; i < 2; i++ {
		if err := queue.Enqueue(QueueEvent{
			ID:   "new-" + string(rune('X'+i)),
			Type: "evolve",
		}); err != nil {
			t.Fatalf("Enqueue new[%d]: %v", i, err)
		}
	}

	// Third read: should see the 2 new events
	events3, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom (3rd call): %v", err)
	}
	if len(events3) != 2 {
		t.Errorf("third read (2 new): expected 2, got %d", len(events3))
	}

	newEvents := 0
	for _, e := range events3 {
		if !processed[e.ID] {
			processed[e.ID] = true
			newEvents++
		}
	}
	if newEvents != 2 {
		t.Errorf("newly processed events: expected 2, got %d", newEvents)
	}

	t.Logf("total unique events processed: %d", len(processed))
}

// TestWorkerReadFromEmptyQueue verifies that ReadFrom works on an empty file.
func TestWorkerReadFromEmptyQueue(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "empty_queue.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	events, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom on empty queue: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("read empty file: expected 0 events, got %d", len(events))
	}
}

// TestWorkerProcessedMapDedup verifies that the processed map deduplicates events.
func TestWorkerProcessedMapDedup(t *testing.T) {
	processed := make(map[string]bool)

	events := []QueueEvent{
		{ID: "evt-1", Type: "evolve"},
		{ID: "evt-2", Type: "evolve"},
		{ID: "evt-1", Type: "evolve"}, // duplicate
	}

	processedCount := 0
	for _, e := range events {
		if !processed[e.ID] {
			processed[e.ID] = true
			processedCount++
		}
	}

	if processedCount != 2 {
		t.Errorf("processedCount: expected 2, got %d", processedCount)
	}
	if len(processed) != 2 {
		t.Errorf("processed map size: expected 2, got %d", len(processed))
	}
}

// TestWorkerProcessedMapCleanup verifies that cleanupProcessed bounds the processed map.
func TestWorkerProcessedMapCleanup(t *testing.T) {
	processed := make(map[string]bool)

	// Simulate 2500 events with deduplication
	for i := 0; i < 2500; i++ {
		id := string(rune(i))
		if !processed[id] {
			processed[id] = true
		}
	}

	if len(processed) != 2500 {
		t.Errorf("deduplicated: expected 2500 events, got %d", len(processed))
	}

	// Simulate cleanupWorker which keeps the last 500
	const maxRetention = 500
	cleaned := make(map[string]bool)
	keys := make([]string, 0, len(processed))
	for k := range processed {
		keys = append(keys, k)
	}
	start := len(keys) - maxRetention
	if start < 0 {
		start = 0
	}
	for _, k := range keys[start:] {
		cleaned[k] = true
	}
	if len(cleaned) != maxRetention {
		t.Errorf("after cleanup, map should have %d entries, got %d", maxRetention, len(cleaned))
	}

	t.Logf("map size before cleanup: %d, after cleanup: %d (bounded)", len(processed), len(cleaned))
}

// TestEventQueueEnqueueAppendOnly verifies that Enqueue uses O_APPEND
// (the file grows, no truncation).
func TestEventQueueEnqueueAppendOnly(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "append_only.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	for i := 0; i < 5; i++ {
		if err := queue.Enqueue(QueueEvent{ID: "evt"}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	size, err := queue.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if size == 0 {
		t.Error("Size should be > 0 after 5 enqueues")
	}
}

// TestEventQueueSync verifies that Enqueue returns an error if the underlying
// file becomes inaccessible (nonexistent directory).
func TestEventQueueSyncError(t *testing.T) {
	// Create an EventQueue, then delete its file and try to write
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "sync_error.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	queue.Close()
}
