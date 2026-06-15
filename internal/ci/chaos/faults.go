package chaos

import (
    "fmt"
    "math/rand"
    "os"
    "time"
)

// Fault represents an injectable failure.
type Fault interface {
    Name() string
    Apply(ctx *Context) error
}

// Context carries runtime execution state for fault injection.
type Context struct {
    WALPath      string
    StepID       string
    JobID        string
    ArtifactPath string
}

// DelayFault injects a random delay.
type DelayFault struct {
    MaxDelayMs int
}

func (f *DelayFault) Name() string { return "delay_fault" }

func (f *DelayFault) Apply(ctx *Context) error {
    d := rand.Intn(f.MaxDelayMs)
    time.Sleep(time.Duration(d) * time.Millisecond)
    return nil
}

// KillWorkerFault simulates a worker crash.
type KillWorkerFault struct{}

func (f *KillWorkerFault) Name() string { return "kill_worker_fault" }

func (f *KillWorkerFault) Apply(ctx *Context) error {
    return fmt.Errorf("simulated worker crash: job=%s step=%s", ctx.JobID, ctx.StepID)
}

// CorruptWALFault injects a corrupted line into the WAL.
type CorruptWALFault struct{}

func (f *CorruptWALFault) Name() string { return "corrupt_wal_fault" }

func (f *CorruptWALFault) Apply(ctx *Context) error {
    fw, err := os.OpenFile(ctx.WALPath, os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    defer fw.Close()

    _, _ = fw.WriteString("{corrupted_event:true}\n")
    return nil
}
