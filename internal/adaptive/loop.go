package adaptive

import (
    "fmt"
    "log"

    "pads-v3/internal/ci"
)

// OracleRef is the canonical reference branch for deterministic validation.
const OracleRef = "ci-oracle-latest"

// Strategy defines a mutation to apply when divergence is detected.
type Strategy func(jobs *map[string]ci.Job, attempt int) error

// Loop is the adaptive execution loop.
// It observes the current state, compares it with the oracle, and retries on divergence.
type Loop struct {
    Scheduler       *ci.Scheduler
    Oracle          *ci.ReplayVerifier
    MaxRetries      int
    Strategy        Strategy
    ReferenceDigest string // optional pre-computed reference digest
}

// Run executes the adaptive loop for the given jobs.
// It returns an error if the loop fails to converge.
func (l *Loop) Run(jobs map[string]ci.Job) error {
    if l.MaxRetries <= 0 {
        l.MaxRetries = 3
    }

    referenceDigest := l.ReferenceDigest
    if referenceDigest == "" {
        return fmt.Errorf("adaptive: reference digest not set")
    }

    for attempt := 1; attempt <= l.MaxRetries; attempt++ {
        log.Printf("adaptive: attempt %d/%d", attempt, l.MaxRetries)

        // 1. OBSERVE : run the current state
        err := l.Scheduler.Run(jobs)
        if err != nil {
            log.Printf("adaptive: execution failed: %v", err)
            if l.Strategy != nil {
                l.Strategy(&jobs, attempt)
            }
            continue
        }

        // 2. COMPARE : compute digest of the produced WAL
        walPath := l.Scheduler.GetWALPath()
        if walPath == "" {
            return fmt.Errorf("adaptive: scheduler has no WAL path")
        }
        producedDigest, err := ci.ComputeDigest(walPath)
        if err != nil {
            log.Printf("adaptive: digest computation failed: %v", err)
            continue
        }

        if producedDigest == referenceDigest {
            log.Printf("adaptive: convergence achieved (attempt %d)", attempt)
            return nil
        }

        log.Printf("adaptive: divergence detected (attempt %d): produced=%s, reference=%s",
            attempt, producedDigest, referenceDigest)

        // 3. RETRY : apply a corrective strategy
        if l.Strategy != nil {
            if err := l.Strategy(&jobs, attempt); err != nil {
                log.Printf("adaptive: strategy error: %v", err)
            }
        } else {
            log.Printf("adaptive: no strategy defined, retrying identical execution")
        }
    }

    return fmt.Errorf("adaptive: failed to converge after %d attempts", l.MaxRetries)
}
