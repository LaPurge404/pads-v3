package ci

import (
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
    _, err := s.RunWithSnapshot(jobs)
    return err
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
