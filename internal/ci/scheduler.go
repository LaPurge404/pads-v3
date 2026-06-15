package ci

import (
    "fmt"

    "pads-v3/internal/ci/chaos"
    "pads-v3/internal/dag"
    "pads-v3/internal/storage"
)

// Scheduler executes a pre-computed DAG deterministically.
type Scheduler struct {
    Cache       Cache
    Artifacts   ArtifactStore
    WAL         *WAL
    DB          *storage.DB
    MaxParallel int
    Chaos       *chaos.Engine // chaos injection engine (nil = disabled)
    ReplayMode  bool          // when true, chaos is disabled for certification
}

// RunWithSnapshot builds the DAG, executes it, and returns the cache snapshot used.
func (s *Scheduler) RunWithSnapshot(jobs map[string]Job) (CacheSnapshot, error) {
    if s.MaxParallel <= 0 {
        s.MaxParallel = 4
    }

    graph, snapshot := BuildDAG(jobs, &s.Cache)

    executor := &dag.Executor{
        Graph:       graph,
        MaxParallel: s.MaxParallel,
    }
    events, err := executor.Run()
    if err != nil {
        return snapshot, err
    }

    // Write all CanonicalEvents to WAL
    for _, ev := range events {
        s.WAL.AppendCanonical(ev)
    }

    return snapshot, nil
}

// Run builds the DAG and executes it.
func (s *Scheduler) Run(jobs map[string]Job) error {
    // Inject chaos before execution if enabled and not in replay mode
    if s.Chaos != nil && !s.ReplayMode {
        s.injectChaos(jobs)
    }

    _, err := s.RunWithSnapshot(jobs)
    return err
}

// injectChaos applies the chaos engine to the execution context.
func (s *Scheduler) injectChaos(jobs map[string]Job) {
    for _, job := range jobs {
        for _, step := range job.Steps {
            ctx := &chaos.Context{
                WALPath: s.WAL.Path(),
                JobID:   job.ID,
                StepID:  step.ID,
            }
            if err := s.Chaos.Inject(ctx); err != nil {
                if s.Chaos.Mode == chaos.ModeHard {
                    // In hard mode, failures are propagated
                    fmt.Printf("chaos: hard fault injected for %s/%s: %v\n", job.ID, step.ID, err)
                }
            }
        }
    }
}

// executePlan executes a Plan (DAG Graph) deterministically.
// Kept for backward compatibility with replay_verifier.
func (s *Scheduler) executePlan(plan *Plan) error {
    executor := &dag.Executor{
        Graph:       plan,
        MaxParallel: s.MaxParallel,
    }
    events, err := executor.Run()
    if err != nil {
        return err
    }
    for _, ev := range events {
        s.WAL.AppendCanonical(ev)
    }
    return nil
}

// GetWALPath returns the path of the scheduler WAL.
func (s *Scheduler) GetWALPath() string {
    if s == nil || s.WAL == nil {
        return ""
    }
    return s.WAL.Path()
}
