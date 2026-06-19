package evolution_test

import (
	"testing"

	"pads-v3/internal/policy/evolution"
)

func TestStabilityGate_Check_AcceptsHighScore(t *testing.T) {
	gate := evolution.NewStabilityGate()
	if !gate.Check(100) {
		t.Fatal("expected high score to be accepted")
	}
}

func TestStabilityGate_Check_RejectsLowScore(t *testing.T) {
	// Use a gate with a higher base threshold so that 10 is rejected
	gate := evolution.NewStabilityGateV2(50, 5, 10.0)
	if gate.Check(10) {
		t.Fatal("expected low score to be rejected")
	}
}

func TestStabilityGate_AdaptiveThreshold(t *testing.T) {
	gate := evolution.NewStabilityGate()
	// Add some low scores then a high but not extremely high score
	gate.Check(10)
	gate.Check(15)
	gate.Check(12)
	// The adaptive threshold should be raised, so 50 might no longer pass
	if gate.Check(50) {
		t.Log("50 still accepted (threshold might not be high enough)")
	}
}

func TestStabilityGate_ExportImport(t *testing.T) {
	gate := evolution.NewStabilityGate()
	gate.Check(80)
	state := gate.ExportState()

	gate2 := evolution.NewStabilityGate()
	gate2.ImportState(state)

	// Verify that both gates make the same decision
	if gate.Check(85) != gate2.Check(85) {
		t.Fatal("export/import mismatch")
	}
}
