package gates

import (
	"context"
	"encoding/json"

	"pads-v3/internal/policy"
)

type SyntaxGate struct{}

func (g *SyntaxGate) Name() string { return "syntax_gate" }

func (g *SyntaxGate) Check(ctx context.Context, input GateInput) policy.GateResult {
	for _, e := range input.Events {
		b, err := json.Marshal(e)
		if err != nil {
			return policy.GateResult{Name: g.Name(), Passed: false, Reason: "invalid canonical event serialization"}
		}
		if len(b) == 0 {
			return policy.GateResult{Name: g.Name(), Passed: false, Reason: "empty event encoding"}
		}
	}
	return policy.GateResult{Name: g.Name(), Passed: true, Reason: "all events syntactically valid"}
}
