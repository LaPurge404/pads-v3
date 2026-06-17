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
}

func NewWorker(q *EventQueue, loop *SafeEvolutionLoopV3, rewarder Rewarder) *Worker {
    return &Worker{
        queue:     q,
        loop:      loop,
        rewarder:  rewarder,
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
                continue
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
