package gates

import (
    "context"

    "pads-v3/internal/event"
)

// GateResult represents a structured gate evaluation outcome.
type GateResult struct {
    Name   string
    Passed bool
    Reason string
}

// Gate defines a CI validation gate.
type Gate interface {
    Name() string
    Check(ctx context.Context, input GateInput) GateResult
}

// GateInput is the shared context for all gates.
type GateInput struct {
    Events       []event.CanonicalEvent
    WALPath      string
    ArtifactPath string
    GraphState   any
}
