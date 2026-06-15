package ci

import "sort"

// CacheSnapshot is a frozen view of the cache at plan time.
type CacheSnapshot map[string]bool

// TakeSnapshot creates a frozen snapshot of the cache's current state.
func (c *Cache) TakeSnapshot(keys []string) CacheSnapshot {
    snap := make(CacheSnapshot)
    for _, key := range keys {
        _, ok := c.Hit(key)
        snap[key] = ok
    }
    return snap
}

// Plan is a fully deterministic execution plan computed before any side effect.
type Plan struct {
    Steps []PlannedStep
}

// PlannedStep is a single step whose execution is fully determined.
type PlannedStep struct {
    JobID      string
    StepID     string
    Run        string
    WorkingDir string
    CacheKey   string
    CacheHit   bool
    Events     []EventRecord
}

// BuildPlan computes a deterministic execution plan from the given jobs and a cache snapshot.
func BuildPlan(jobs map[string]Job, cache *Cache) (*Plan, CacheSnapshot) {
    // First, collect all potential cache keys
    var allKeys []string
    order, _ := ResolveJobOrder(jobs)

    for _, job := range order {
        steps := make([]Step, len(job.Steps))
        copy(steps, job.Steps)
        sort.Slice(steps, func(i, j int) bool {
            return steps[i].ID < steps[j].ID
        })
        matrix := job.Matrix.Expand()
        for _, m := range matrix {
            varsCopy := cloneMap(m)
            matrixHash := flatten(varsCopy)
            input := flatten(varsCopy)
            for _, step := range steps {
                key := cache.Key(job.ID, step, input, matrixHash)
                allKeys = append(allKeys, key)
            }
        }
    }

    // Take a frozen snapshot of the cache
    snapshot := cache.TakeSnapshot(allKeys)

    // Build the plan using the snapshot
    var plan Plan
    for _, job := range order {
        plan.Steps = append(plan.Steps, PlannedStep{
            JobID: job.ID,
            Events: []EventRecord{
                {Type: "CI_JOB_STARTED", JobID: job.ID, Status: "RUNNING"},
            },
        })

        steps := make([]Step, len(job.Steps))
        copy(steps, job.Steps)
        sort.Slice(steps, func(i, j int) bool {
            return steps[i].ID < steps[j].ID
        })
        matrix := job.Matrix.Expand()
        for _, m := range matrix {
            varsCopy := cloneMap(m)
            matrixHash := flatten(varsCopy)
            for _, step := range steps {
                input := flatten(varsCopy)
                key := cache.Key(job.ID, step, input, matrixHash)
                cacheHit := snapshot[key]

                events := []EventRecord{
                    {Type: "CI_STEP_STARTED", JobID: job.ID, StepID: step.ID, Status: "RUNNING"},
                }
                if cacheHit {
                    events = append(events, EventRecord{
                        Type: "CI_CACHE_HIT", JobID: job.ID, StepID: step.ID, Status: "CACHED",
                    })
                } else {
                    events = append(events, EventRecord{
                        Type: "CI_CACHE_MISS", JobID: job.ID, StepID: step.ID, Status: "MISS",
                    })
                }

                plan.Steps = append(plan.Steps, PlannedStep{
                    JobID:      job.ID,
                    StepID:     step.ID,
                    Run:        step.Run,
                    WorkingDir: step.WorkingDir,
                    CacheKey:   key,
                    CacheHit:   cacheHit,
                    Events:     events,
                })
            }
        }

        plan.Steps = append(plan.Steps, PlannedStep{
            JobID: job.ID,
            Events: []EventRecord{
                {Type: "CI_JOB_FINISHED", JobID: job.ID, Status: "SUCCESS"},
            },
        })
    }

    return &plan, snapshot
}
