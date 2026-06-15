package runner

import (
    "context"
    "time"

    "pads-v3/internal/ci/certification"
    "pads-v3/internal/ci/gates"
)

// CIReport aggregates the results of a complete CI run.
type CIReport struct {
    RunID         string
    Deterministic bool
    Duration      time.Duration
    Gates         []gates.GateResult
    Certificate   *certification.Certificate
    ChaosEvents   []string
    Passed        bool
    Reason        string
}

// GateRunner executes all registered gates and returns a CIReport.
type GateRunner struct {
    Gates []gates.Gate
}

// Run executes the gates against the given input and produces a report.
func (gr *GateRunner) Run(input gates.GateInput, cert *certification.Certificate, chaosEvents []string) CIReport {
    start := time.Now()
    var results []gates.GateResult
    allPassed := true
    reason := "all gates passed"

    for _, gate := range gr.Gates {
        res := gate.Check(context.Background(), input)
        results = append(results, res)
        if !res.Passed {
            allPassed = false
            reason = res.Reason
        }
    }

    deterministic := cert != nil && cert.Deterministic

    return CIReport{
        RunID:         cert.RunID,
        Deterministic: deterministic,
        Duration:      time.Since(start),
        Gates:         results,
        Certificate:   cert,
        ChaosEvents:   chaosEvents,
        Passed:        allPassed && deterministic,
        Reason:        reason,
    }
}
