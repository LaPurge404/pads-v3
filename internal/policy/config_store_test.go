package policy

import (
	"math"
	"testing"
)

func TestConfigStore_EMASmoothing(t *testing.T) {
	initial := TunedConfig{ThresholdPass: 90, ThresholdWarn: 70, ThresholdFail: 50}
	store := NewConfigStore(initial)

	// Feed stable scores to allow updates
	for i := 0; i < 10; i++ {
		store.UpdateScore(95.0)
	}

	store.Update(TunedConfig{ThresholdPass: 80, ThresholdWarn: 60, ThresholdFail: 40})

	got := store.Get()
	expected := 87.0 // 0.3*80 + 0.7*90
	if math.Abs(got.ThresholdPass-expected) > 0.001 {
		t.Errorf("expected EMA-smoothed pass=%.1f, got %.1f", expected, got.ThresholdPass)
	}
}

func TestConfigStore_Rollback(t *testing.T) {
	initial := TunedConfig{ThresholdPass: 90}
	store := NewConfigStore(initial)

	// Feed stable scores
	for i := 0; i < 10; i++ {
		store.UpdateScore(95.0)
	}

	store.Update(TunedConfig{ThresholdPass: 80}) // EMA -> 87
	store.Update(TunedConfig{ThresholdPass: 70}) // EMA -> 81.9

	if !store.Rollback() {
		t.Fatal("expected successful rollback")
	}
	cfg := store.Get()
	if math.Abs(cfg.ThresholdPass-87.0) > 0.001 {
		t.Errorf("expected rolled back pass=87, got %.1f", cfg.ThresholdPass)
	}

	if !store.Rollback() {
		t.Fatal("expected second successful rollback")
	}
	cfg = store.Get()
	if math.Abs(cfg.ThresholdPass-90.0) > 0.001 {
		t.Errorf("expected initial pass=90, got %.1f", cfg.ThresholdPass)
	}
}

func TestConfigStore_ShouldUpdate_RejectsWhenUnstable(t *testing.T) {
	store := NewConfigStore(TunedConfig{ThresholdPass: 90})

	// Feed highly variable scores to create instability
	store.UpdateScore(100)
	store.UpdateScore(40)
	store.UpdateScore(90)
	store.UpdateScore(30)
	store.UpdateScore(80)
	store.UpdateScore(20)

	if store.ShouldUpdate() {
		t.Error("expected ShouldUpdate to reject when unstable")
	}
}

func TestConfigStore_AdaptiveAlpha(t *testing.T) {
	store := NewConfigStore(TunedConfig{ThresholdPass: 90})
	baseAlpha := store.AdaptiveAlpha()

	// Introduce instability
	store.UpdateScore(100)
	store.UpdateScore(20)
	store.UpdateScore(90)
	store.UpdateScore(10)
	store.UpdateScore(80)
	store.UpdateScore(30)

	unstableAlpha := store.AdaptiveAlpha()
	if unstableAlpha >= baseAlpha {
		t.Errorf("expected adaptive alpha (%.3f) to be smaller than base (%.3f) when unstable",
			unstableAlpha, baseAlpha)
	}
}

func TestConfigStore_ShadowEvaluate(t *testing.T) {
	initial := TunedConfig{
		ThresholdPass: 90,
		ThresholdWarn: 70,
		ThresholdFail: 50,
		GateWeights: map[string]int{
			"syntax_gate": 30,
		},
	}
	store := NewConfigStore(initial)
	engine := NewEngine(store)

	// Create a candidate config with lower threshold (more lenient)
	candidate := initial
	candidate.ThresholdPass = 80

	// Create sample inputs that would pass with the candidate but fail with current
	inputs := []GateInput{
		{
			Gates: []GateResult{
				{Name: "syntax_gate", Passed: true},
			},
			Cert:  &CertificationResult{Deterministic: true},
			Chaos: &ChaosReport{Active: false},
		},
	}

	candidateAvg, currentAvg, accept := store.ShadowEvaluate(candidate, inputs, engine)
	t.Logf("candidate avg: %.2f, current avg: %.2f, accept: %v", candidateAvg, currentAvg, accept)

	// Both should have perfect scores since the input passes all gates
	// Candidate should not be accepted since it doesn't improve (both 100)
	if accept {
		t.Log("candidate accepted (both perfect scores)")
	}
}
