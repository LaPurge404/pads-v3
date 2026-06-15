package gates

import (
    "context"
    "encoding/json"
)

type SyntaxGate struct{}

func (g *SyntaxGate) Name() string { return "syntax_gate" }

func (g *SyntaxGate) Check(ctx context.Context, input GateInput) GateResult {
    for _, e := range input.Events {
        b, err := json.Marshal(e)
        if err != nil {
            return GateResult{Name: g.Name(), Passed: false, Reason: "invalid canonical event serialization"}
        }
        if len(b) == 0 {
            return GateResult{Name: g.Name(), Passed: false, Reason: "empty event encoding"}
        }
    }
    return GateResult{Name: g.Name(), Passed: true, Reason: "all events syntactically valid"}
}
