package gates

import (
	"context"

	"pads-v3/internal/event"
	"pads-v3/internal/policy"
)

// Gate defines a CI validation gate.
type Gate interface {
	Name() string
	Check(ctx context.Context, input GateInput) policy.GateResult
}

// GateInput is the shared context for all gates.
type GateInput struct {
	Events       []event.CanonicalEvent
	WALPath      string
	ArtifactPath string
	GraphState   any
}
