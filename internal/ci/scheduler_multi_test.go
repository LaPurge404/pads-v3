package ci

import (
    "testing"

    "pads-v3/internal/trace"
)

func TestScheduler_MultiJob(t *testing.T) {
    tmpDir := t.TempDir()
    walPath := tmpDir + "/ci.wal"

    wal, _ := NewWAL(walPath)
    sched := &Scheduler{
        Cache:       NewCache(tmpDir + "/cache"),
        Artifacts:   NewArtifactStore(nil, tmpDir+"/artifacts", wal),
        WAL:         wal,
        MaxParallel: 2,
    }
    jobs := map[string]Job{
        "job1": {
            ID: "job1",
            Steps: []Step{
                {ID: "stepA", Run: "echo hello"},
            },
        },
        "job2": {
            ID: "job2",
            Needs: []string{"job1"},
            Steps: []Step{
                {ID: "stepB", Run: "echo world"},
            },
        },
    }
    if err := sched.Run(jobs); err != nil {
        t.Fatal(err)
    }
    wal.Close()

    // Verify WAL contains events from both jobs
    events, err := trace.ReadWALFile(walPath)
    if err != nil {
        t.Fatal(err)
    }
    hasJob1 := false
    hasJob2 := false
    for _, e := range events {
        if e.JobID == "job1" {
            hasJob1 = true
        }
        if e.JobID == "job2" {
            hasJob2 = true
        }
    }
    if !hasJob1 || !hasJob2 {
        t.Errorf("missing job events: job1=%v job2=%v", hasJob1, hasJob2)
    }
}
