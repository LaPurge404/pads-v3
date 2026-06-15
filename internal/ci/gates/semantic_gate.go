package gates

import (
    "context"

    "pads-v3/internal/policy"
)

type SemanticGate struct{}

func (g *SemanticGate) Name() string { return "semantic_gate" }

func (g *SemanticGate) Check(ctx context.Context, input GateInput) policy.GateResult {
    jobSeen := make(map[string]bool)
    for _, e := range input.Events {
        if e.JobID == "" {
            return policy.GateResult{Name: g.Name(), Passed: false, Reason: "missing job ID in event"}
        }
        jobSeen[e.JobID] = true
    }
    if len(jobSeen) == 0 {
        return policy.GateResult{Name: g.Name(), Passed: false, Reason: "no jobs found in event stream"}
    }
    return policy.GateResult{Name: g.Name(), Passed: true, Reason: "semantic structure valid"}
}
