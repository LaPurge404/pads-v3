package evolution

import (
	"fmt"
	"log/slog"
	"time"
)

// worker.go — uses EventQueue with internal offset tracking.
type Worker struct {
	queue        *EventQueue
	loop         *SafeEvolutionLoopV3
	rewarder     Rewarder
	Running      bool
	concurrency  int
	processed    map[string]bool
	processedOrd []string // ordered list for LRU-like cleanup
	processedCnt int      // compteur pour nettoyage périodique
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
	w.Running = true
	slog.Info("worker démarré")
	for w.Running {
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

			// Cleanup périodique de la map processed pour éviter une croissance infinie
			if w.processedCnt >= processedCleanupThreshold {
				w.cleanupProcessed()
				w.processedCnt = 0
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// cleanupProcessed supprime les entrées anciennes de la map processed.
// On garde les maxProcessedRetention dernières entrées pour maintenir la déduplication.
// Les entrées plus anciennes sont supprimées (répétition possible en cas de crash restart,
// ce qui est acceptable car le traitement est idempotent).
func (w *Worker) cleanupProcessed() {
	if len(w.processedOrd) <= maxProcessedRetention {
		slog.Debug("worker: cleanup skipped, below threshold", "size", len(w.processed))
		return
	}

	// Identifier les IDs à supprimer (les plus anciens)
	toRemove := w.processedOrd[:len(w.processedOrd)-maxProcessedRetention]

	// Supprimer de la map
	for _, id := range toRemove {
		delete(w.processed, id)
	}

	// Garder seulement les derniers
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
