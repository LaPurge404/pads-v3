package gates

import (
	"context"
	"os"

	"pads-v3/internal/policy"
)

type ExecutionGate struct{}

func (g *ExecutionGate) Name() string { return "execution_gate" }

func (g *ExecutionGate) Check(ctx context.Context, input GateInput) policy.GateResult {
	if input.WALPath == "" {
		return policy.GateResult{Name: g.Name(), Passed: false, Reason: "missing WAL path"}
	}
	if _, err := os.Stat(input.WALPath); err != nil {
		return policy.GateResult{Name: g.Name(), Passed: false, Reason: "WAL file missing or unreadable"}
	}
	return policy.GateResult{Name: g.Name(), Passed: true, Reason: "execution artifacts present"}
}
