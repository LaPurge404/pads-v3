package gates

import (
	"context"
	"testing"

	"pads-v3/internal/event"
)

func TestSyntaxGate(t *testing.T) {
	g := &SyntaxGate{}
	input := GateInput{
		Events: []event.CanonicalEvent{
			{Type: "A", JobID: "j1"},
		},
	}
	res := g.Check(context.Background(), input)
	if !res.Passed {
		t.Fatalf("expected syntax gate to pass: %s", res.Reason)
	}
}

func TestSemanticGate(t *testing.T) {
	g := &SemanticGate{}
	input := GateInput{
		Events: []event.CanonicalEvent{
			{Type: "A", JobID: "j1"},
			{Type: "B", JobID: "j2"},
		},
	}
	res := g.Check(context.Background(), input)
	if !res.Passed {
		t.Fatalf("semantic gate failed: %s", res.Reason)
	}
}
