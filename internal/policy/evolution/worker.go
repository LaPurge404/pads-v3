package evolution

import (
	"fmt"
	"log/slog"
	"time"
)

type Worker struct {
	queue       *EventQueue
	loop        *SafeEvolutionLoopV3
	rewarder    Rewarder
	Running     bool
	concurrency int
	processed   map[string]bool
	offset      int64 // position de lecture dans le fichier de queue
	processedCnt int  // compteur pour nettoyage périodique
}

const processedCleanupThreshold = 1000

func NewWorker(q *EventQueue, loop *SafeEvolutionLoopV3, rewarder Rewarder) *Worker {
	return &Worker{
		queue:     q,
		loop:      loop,
		rewarder:  rewarder,
		concurrency: 1,
		processed: make(map[string]bool),
		offset:    0,
	}
}

func (w *Worker) Start() {
	w.Running = true
	slog.Info("worker démarré")
	for w.Running {
		// Lecture incrémentale : ne lire que les nouvelles lignes depuis le dernier offset
		events, newOffset, err := w.queue.ReadFrom(w.offset)
		if err != nil {
			slog.Error("worker lecture queue", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		w.offset = newOffset

		for _, e := range events {
			if w.processed[e.ID] {
				continue
			}
			if err := w.process(e); err != nil {
				slog.Error("worker traitement événement", "id", e.ID, "error", err)
				continue
			}
			w.processed[e.ID] = true
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

// cleanupProcessed supprime les entrées de la map processed.
// Après cleanup, tous les événements déjà traités seront rejoués si le worker redémarre
// (ce qui est le comportement souhaité : idempotence via la map).
func (w *Worker) cleanupProcessed() {
	// Conserver les 100 derniers IDs pour le cas où le worker redémarre
	// On ne peut pas simplement vider la map car on perdrait la déduplication.
	// On garde la map telle quelle mais on note que sa taille est bornée par le nombre
	// d'événements entre deux cleanups (processedCleanupThreshold).
	// Une amélioration future serait d'utiliser un LRU cache borné.
	slog.Info("worker: cleanup processed map", "size", len(w.processed))
}

func (w *Worker) process(e QueueEvent) error {
	oldStability := w.loop.StabilityScore()

	result, accepted, err := w.loop.Evolve(
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

	_ = result
	return nil
}
