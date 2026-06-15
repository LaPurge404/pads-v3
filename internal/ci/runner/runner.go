package runner

import (
    "context"
    "time"

    "pads-v3/internal/ci/certification"
    "pads-v3/internal/ci/gates"
    "pads-v3/internal/policy"
    "pads-v3/internal/policy/learner"
    "pads-v3/internal/policy/shadow"
    "pads-v3/internal/policy/wal"
)

type CIReport struct {
    RunID         string
    Deterministic bool
    Duration      time.Duration
    Gates         []policy.GateResult
    Certificate   *certification.Certificate
    ChaosEvents   []string
    Passed        bool
    Reason        string
}

type GateRunner struct {
    Gates []gates.Gate
}

func (gr *GateRunner) Run(input gates.GateInput, cert *certification.Certificate, chaosEvents []string) CIReport {
    start := time.Now()
    var results []policy.GateResult
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

type PolicyRunner struct {
    GateRunner      *GateRunner
    PolicyEngine    *policy.Engine
    ExplainEngine   *policy.ExplainEngine
    PolicyWAL       *wal.PolicyWAL
    Learner         *learner.Learner
    ConfigStore     *policy.ConfigStore
    ShadowEvaluator *shadow.ShadowEvaluator

    recentInputs  []policy.GateInput
    maxBufferSize int
}

func (pr *PolicyRunner) Run(input gates.GateInput, cert *certification.Certificate, chaosEvents []string) (policy.PolicyDecision, error) {
    report := pr.GateRunner.Run(input, cert, chaosEvents)

    certResult := &policy.CertificationResult{
        Deterministic: cert != nil && cert.Deterministic,
        WALHash:       cert.WALHash,
        ReplayHash:    cert.ReplayHash,
    }

    chaosReport := &policy.ChaosReport{
        Active: len(chaosEvents) > 0,
        Mode:   "Silent",
        Events: chaosEvents,
    }

    decision := pr.PolicyEngine.Evaluate(report.Gates, certResult, chaosReport)
    trace := pr.ExplainEngine.BuildTrace(report.Gates, certResult, chaosReport, decision)
    explanation := policy.BuildExplanation(trace)

    event := wal.PolicyEvent{
        DecisionID:  trace.DecisionID,
        Score:       decision.Score,
        Status:      string(decision.Status),
        Trace:       trace,
        Explanation: explanation,
    }
    if pr.PolicyWAL != nil {
        _ = pr.PolicyWAL.Append(event)
    }

    pr.bufferInput(policy.GateInput{
        Gates: report.Gates,
        Cert:  certResult,
        Chaos: chaosReport,
    })

    if pr.ConfigStore != nil {
        pr.ConfigStore.UpdateScore(decision.Score)
    }

    if pr.Learner != nil && pr.PolicyWAL != nil && pr.Learner.ShouldLearn() {
        if pr.ConfigStore != nil && pr.ConfigStore.ShouldUpdate() && pr.ShadowEvaluator != nil {
            go func() {
                tuned, _, err := pr.Learner.LearnFromWAL(pr.PolicyWAL, pr.ConfigStore.Get())
                if err != nil || tuned == nil {
                    return
                }

                _, _, accepted := pr.ShadowEvaluator.Evaluate(
                    *tuned,
                    pr.ConfigStore.Get(),
                    pr.getRecentInputs(),
                    pr.PolicyEngine,
                )
                if accepted {
                    pr.ConfigStore.Update(*tuned)
                }
            }()
        }
    }

    return decision, nil
}

func (pr *PolicyRunner) bufferInput(input policy.GateInput) {
    if pr.maxBufferSize == 0 {
        pr.maxBufferSize = 20
    }
    pr.recentInputs = append(pr.recentInputs, input)
    if len(pr.recentInputs) > pr.maxBufferSize {
        pr.recentInputs = pr.recentInputs[len(pr.recentInputs)-pr.maxBufferSize:]
    }
}

func (pr *PolicyRunner) getRecentInputs() []policy.GateInput {
    return pr.recentInputs
}
