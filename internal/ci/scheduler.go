package ci

import (
    "fmt"
    "sort"
    "sync"

    "pads-v3/internal/storage"
)

// Scheduler executes a pre-computed Plan deterministically.
type Scheduler struct {
    Cache       Cache
    Artifacts   ArtifactStore
    WAL         *WAL
    DB          *storage.DB
    MaxParallel int
}

// RunWithSnapshot builds the plan, executes it, and returns the cache snapshot used.
func (s *Scheduler) RunWithSnapshot(jobs map[string]Job) (CacheSnapshot, error) {
    if s.MaxParallel <= 0 {
        s.MaxParallel = 4
    }

    plan, snapshot := BuildPlan(jobs, &s.Cache)

    return snapshot, s.executePlan(plan)
}

// Run builds the plan and executes it (backward-compatible).
func (s *Scheduler) Run(jobs map[string]Job) error {
    _, err := s.RunWithSnapshot(jobs)
    return err
}

// executePlan runs the planned steps under strict constraints.
func (s *Scheduler) executePlan(plan *Plan) error {
    eventCh := make(chan EventRecord, 1000)
    writerDone := make(chan struct{})
    go func() {
        defer close(writerDone)
        for e := range eventCh {
            _, _ = s.WAL.Append(e)
        }
    }()

    var mu sync.Mutex
    var firstErr error
    var wg sync.WaitGroup
    sem := make(chan struct{}, s.MaxParallel)

    for _, ps := range plan.Steps {
        if ps.StepID == "" {
            for _, e := range ps.Events {
                eventCh <- e
            }
            continue
        }

        wg.Add(1)
        go func(ps PlannedStep) {
            defer wg.Done()
            defer func() {
                if r := recover(); r != nil {
                    mu.Lock()
                    if firstErr == nil {
                        firstErr = fmt.Errorf("panic in step %s/%s: %v", ps.JobID, ps.StepID, r)
                    }
                    mu.Unlock()
                }
                <-sem
            }()
            sem <- struct{}{}

            for _, e := range ps.Events {
                eventCh <- e
            }

            if ps.CacheHit {
                out, _ := s.Cache.Hit(ps.CacheKey)
                _ = s.Artifacts.Save(ps.JobID, ps.StepID, "CACHE:\n"+out)
                eventCh <- EventRecord{
                    Type: "CI_STEP_FINISHED", JobID: ps.JobID, StepID: ps.StepID, Status: "SUCCESS",
                }
                return
            }

            step := Step{Run: ps.Run, WorkingDir: ps.WorkingDir}
            res, err := RunStep(step)
            if err != nil || res.ExitCode != 0 {
                eventCh <- EventRecord{
                    Type: "CI_STEP_FINISHED", JobID: ps.JobID, StepID: ps.StepID, Status: "FAILED",
                }
                mu.Lock()
                if firstErr == nil {
                    firstErr = fmt.Errorf("step %s/%s failed: exit %d", ps.JobID, ps.StepID, res.ExitCode)
                }
                mu.Unlock()
                return
            }

            _ = s.Cache.Store(ps.CacheKey, res.Output)
            _ = s.Artifacts.Save(ps.JobID, ps.StepID, res.Output)
            eventCh <- EventRecord{
                Type: "CI_STEP_FINISHED", JobID: ps.JobID, StepID: ps.StepID, Status: "SUCCESS",
            }
        }(ps)
    }

    wg.Wait()
    close(eventCh)
    <-writerDone

    return firstErr
}

var _ = sort.Strings
