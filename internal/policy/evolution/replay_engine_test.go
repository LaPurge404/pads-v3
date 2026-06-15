package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestReplayEngine_Rebuild(t *testing.T) {
    events := []evolution.Event{
        {Sequence: 1, CandidateScore: 70, CurrentScore: 50, Weight: 1.0, Mode: evolution.ModeStable},
        {Sequence: 2, CandidateScore: 30, CurrentScore: 80, Weight: 1.0, Mode: evolution.ModeStable},
    }
    engine := evolution.NewReplayEngine(events)
    state := engine.Rebuild()
    if state.Sequence != 2 {
        t.Fatalf("expected sequence 2, got %d", state.Sequence)
    }
    if len(state.DetectorWindow) != 2 {
        t.Fatalf("expected detector window length 2, got %d", len(state.DetectorWindow))
    }
}
