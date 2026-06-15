package ci

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "os"
    "sort"
    "strings"
)

// ReplayVerifier replays a WAL file and verifies determinism.
type ReplayVerifier struct {
    Cache       Cache
    ArtifactsDir string // base directory for replay artifacts
}

// ReplayResult contains the outcome of a replay verification.
type ReplayResult struct {
    OK          bool
    OriginalLen int
    ReplayLen   int
    FirstDiff   int   // -1 if no diff found
    OriginalSeq []EventRecord
    ReplaySeq   []EventRecord
}

// Replay replays a run using the given cache snapshot and compares output hashes.
func (rv *ReplayVerifier) Replay(walPath string, jobs map[string]Job, snapshot CacheSnapshot) (ReplayResult, error) {
    origEvents, err := readWALFile(walPath)
    if err != nil {
        return ReplayResult{}, fmt.Errorf("read original WAL: %w", err)
    }

    replayWalPath := walPath + ".replay"
    replayWal, err := NewWAL(replayWalPath)
    if err != nil {
        return ReplayResult{}, fmt.Errorf("create replay WAL: %w", err)
    }
    defer replayWal.Close()

    // Create an ArtifactStore that writes to the replay WAL
    artifactDir, err := os.MkdirTemp(rv.ArtifactsDir, "replay-artifacts-*")
    if err != nil {
        return ReplayResult{}, fmt.Errorf("create artifact dir: %w", err)
    }
    defer os.RemoveAll(artifactDir)
    artifacts := NewArtifactStore(nil, artifactDir, replayWal)

    plan := BuildPlanFromSnapshot(jobs, &rv.Cache, snapshot)
    sched := &Scheduler{
        Cache:       rv.Cache,
        Artifacts:   artifacts, // use artifact store that writes to replay WAL
        WAL:         replayWal,
        MaxParallel: 1,
    }
    if err := sched.executePlan(plan); err != nil {
        return ReplayResult{}, fmt.Errorf("replay execution: %w", err)
    }
    replayWal.Close()

    replayEvents, err := readWALFile(replayWalPath)
    if err != nil {
        return ReplayResult{}, fmt.Errorf("read replay WAL: %w", err)
    }

    result := ReplayResult{
        OriginalLen: len(origEvents),
        ReplayLen:   len(replayEvents),
        OriginalSeq: origEvents,
        ReplaySeq:   replayEvents,
        OK:          true,
        FirstDiff:   -1,
    }

    if len(origEvents) != len(replayEvents) {
        result.OK = false
        return result, nil
    }

    for i := range origEvents {
        if !eventsEqual(origEvents[i], replayEvents[i]) {
            result.OK = false
            result.FirstDiff = i
            break
        }
    }

    return result, nil
}

// eventsEqual compares two EventRecords for logical equality using hash comparison.
func eventsEqual(a, b EventRecord) bool {
    if a.Type != b.Type || a.JobID != b.JobID || a.StepID != b.StepID || a.Status != b.Status {
        return false
    }
    return hashString(a.Payload) == hashString(b.Payload)
}

// hashString returns the SHA-256 hash of a string.
func hashString(s string) string {
    sum := sha256.Sum256([]byte(s))
    return fmt.Sprintf("%x", sum)
}

// BuildPlanFromSnapshot builds a plan deterministically from a given cache snapshot.
func BuildPlanFromSnapshot(jobs map[string]Job, cache *Cache, snapshot CacheSnapshot) *Plan {
    order, _ := ResolveJobOrder(jobs)
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
        sort.Slice(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
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
    return &plan
}

func readWALFile(path string) ([]EventRecord, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    lines := strings.Split(strings.TrimSpace(string(data)), "\n")
    var events []EventRecord
    for _, line := range lines {
        if line == "" {
            continue
        }
        var e EventRecord
        if err := json.Unmarshal([]byte(line), &e); err != nil {
            return nil, fmt.Errorf("json unmarshal: %w (line: %s)", err, line)
        }
        events = append(events, e)
    }
    return events, nil
}
