package evolution

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// worker.go — uses EventQueue with internal offset tracking.
type Worker struct {
	queue       *EventQueue
	loop        *SafeEvolutionLoopV3
	rewarder    Rewarder
	running     bool
	runningMu   sync.RWMutex // guards `running`; use IsRunning() for reads, Start()/Stop() for writes
	concurrency int
	processed   map[string]bool
	// ordered list for LRU-like cleanup
	processedOrd []string
	// counter for periodic cleanup
	processedCnt int
}

const (
	processedCleanupThreshold = 1000
	maxProcessedRetention     = 500 // keep last 500 entries for deduplication
)

func NewWorker(q *EventQueue, loop *SafeEvolutionLoopV3, rewarder Rewarder) *Worker {
	return &Worker{
		queue:        q,
		loop:         loop,
		rewarder:     rewarder,
		concurrency:  1,
		processed:    make(map[string]bool),
		processedOrd: make([]string, 0, processedCleanupThreshold),
	}
}

func (w *Worker) Start() {
	w.runningMu.Lock()
	if w.running {
		w.runningMu.Unlock()
		slog.Warn("worker.Start déjà en cours, appel ignoré")
		return
	}
	w.running = true
	w.runningMu.Unlock()
	slog.Info("worker démarré")
	for w.IsRunning() {
		// ReadFrom tracks offset internally — reads only new events since last call
		events, err := w.queue.ReadFrom()
		if err != nil {
			slog.Error("worker lecture queue", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		for _, e := range events {
			if w.processed[e.ID] {
				continue
			}
			if err := w.process(e); err != nil {
				slog.Error("worker traitement événement", "id", e.ID, "error", err)
				continue
			}
			w.processed[e.ID] = true
			w.processedOrd = append(w.processedOrd, e.ID)
			w.processedCnt++

			// Periodic cleanup of the processed map to avoid unbounded growth
			if w.processedCnt >= processedCleanupThreshold {
				w.cleanupProcessed()
				w.processedCnt = 0
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	slog.Info("worker arrêté")
}

// Stop requests the worker loop to exit. Safe to call from any goroutine,
// including concurrent callers; the loop observes the flag in at most one
// iteration (≤ 500ms) via IsRunning(). Returns true if this call actually
// flipped the flag from running→stopped, false if the worker was already
// stopped (or returning after an idempotent re-call).
func (w *Worker) Stop() bool {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()
	if !w.running {
		return false
	}
	w.running = false
	return true
}

// IsRunning reports whether the worker goroutine is active. Safe to call
// from any goroutine — used by the /health handler.
func (w *Worker) IsRunning() bool {
	w.runningMu.RLock()
	defer w.runningMu.RUnlock()
	return w.running
}

// cleanupProcessed removes stale entries from the processed map.
// We keep the last maxProcessedRetention entries to maintain deduplication.
// Older entries are removed (re-processing may occur after crash restart,
// which is acceptable as the processing is idempotent).
func (w *Worker) cleanupProcessed() {
	if len(w.processedOrd) <= maxProcessedRetention {
		slog.Debug("worker: cleanup skipped, below threshold", "size", len(w.processed))
		return
	}

	// Identify IDs to remove (oldest ones)
	toRemove := w.processedOrd[:len(w.processedOrd)-maxProcessedRetention]

	// Remove from the map
	for _, id := range toRemove {
		delete(w.processed, id)
	}

	// Keep only the last entries
	w.processedOrd = w.processedOrd[len(toRemove):]

	slog.Info("worker: cleanup processed map", "removed", len(toRemove), "remaining", len(w.processedOrd))
}

func (w *Worker) process(e QueueEvent) error {
	oldStability := w.loop.StabilityScore()

	accepted, err := w.loop.Evolve(
		Candidate{Score: e.Candidate},
		Candidate{Score: e.Current},
		e.Weight,
	)
	if err != nil {
		return fmt.Errorf("worker evolve failed: %w", err)
	}

	newStability := w.loop.StabilityScore()

	// Apprentissage
	if w.loop.selector != nil && w.rewarder != nil {
		reward := w.rewarder.ComputeReward(oldStability, newStability, accepted)
		w.loop.selector.Update(string(e.Mode), reward)
	}

	return nil
}
