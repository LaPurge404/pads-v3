package ci

import (
    "os/exec"
    "time"
)

// StepResult is the outcome of running a single step.
type StepResult struct {
    ExitCode int
    Output   string
    Duration time.Duration
}

// RunStep executes a step using sh -c.
func RunStep(step Step) (StepResult, error) {
    start := time.Now()
    cmd := exec.Command("sh", "-c", step.Run)
    if step.WorkingDir != "" {
        cmd.Dir = step.WorkingDir
    }
    out, err := cmd.CombinedOutput()
    duration := time.Since(start)

    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return StepResult{
                ExitCode: exitErr.ExitCode(),
                Output:   string(out),
                Duration: duration,
            }, nil
        }
        return StepResult{ExitCode: 1, Output: string(out), Duration: duration}, err
    }
    return StepResult{ExitCode: 0, Output: string(out), Duration: duration}, nil
}
