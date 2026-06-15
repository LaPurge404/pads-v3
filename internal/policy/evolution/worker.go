package evolution

import (
    "fmt"
    "log/slog"
    "time"
)

type Worker struct {
    queue       *EventQueue
    loop        *SafeEvolutionLoopV3
    Running     bool
    concurrency int
    // Suivi des IDs déjà traités pour éviter les doubles exécutions
    processed map[string]bool
}

func NewWorker(q *EventQueue, loop *SafeEvolutionLoopV3) *Worker {
    return &Worker{
        queue:     q,
        loop:      loop,
        concurrency: 1,
        processed: make(map[string]bool),
    }
}

func (w *Worker) Start() {
    w.Running = true
    slog.Info("worker démarré")
    for w.Running {
        events, err := w.queue.LoadAll()
        if err != nil {
            slog.Error("worker chargement queue", "error", err)
            time.Sleep(1 * time.Second)
            continue
        }

        for _, e := range events {
            if w.processed[e.ID] {
                continue // déjà traité
            }
            if err := w.process(e); err != nil {
                slog.Error("worker traitement événement", "id", e.ID, "error", err)
                continue
            }
            w.processed[e.ID] = true
        }
        time.Sleep(500 * time.Millisecond)
    }
}

func (w *Worker) process(e QueueEvent) error {
    _, _, err := w.loop.Evolve(
        Candidate{Score: e.Candidate},
        Candidate{Score: e.Current},
        e.Weight,
    )
    if err != nil {
        return fmt.Errorf("worker evolve failed: %w", err)
    }
    return nil
}
