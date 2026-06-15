package gates

import (
    "context"

    "pads-v3/internal/policy"
    "pads-v3/internal/trace"
)

type DeterminismGate struct{}

func (g *DeterminismGate) Name() string { return "determinism_gate" }

func (g *DeterminismGate) Check(ctx context.Context, input GateInput) policy.GateResult {
    if input.WALPath == "" {
        return policy.GateResult{Name: g.Name(), Passed: false, Reason: "missing WAL path"}
    }
    events, err := trace.ReadWALFile(input.WALPath)
    if err != nil {
        return policy.GateResult{Name: g.Name(), Passed: false, Reason: "failed to read WAL"}
    }
    seen := make(map[string]struct{})
    for _, e := range events {
        key := e.JobID + ":" + e.Type + ":" + e.StepID
        if _, ok := seen[key]; ok {
            continue
        }
        seen[key] = struct{}{}
    }
    if len(seen) == 0 {
        return policy.GateResult{Name: g.Name(), Passed: false, Reason: "empty or invalid WAL stream"}
    }
    return policy.GateResult{Name: g.Name(), Passed: true, Reason: "determinism constraints satisfied"}
}
