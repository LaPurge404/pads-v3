package change

import (
	"testing"

	"pads-v3/internal/policy"
)

func engineFactory(cfg policy.TunedConfig) *policy.Engine {
	return policy.NewEngine(policy.NewConfigStore(cfg))
}

func TestValidator_AcceptsBetterCandidate(t *testing.T) {
	current := policy.TunedConfig{
		ThresholdPass: 90,
		ThresholdWarn: 70,
		ThresholdFail: 50,
		GateWeights: map[string]int{
			"syntax_gate":    30,
			"execution_gate": 70,
		},
		HardFailGates: map[string]bool{
			"execution_gate": true,
		},
		ChaosPenalties: map[string]float64{
			"Silent": 5,
		},
	}

	candidate := current
	candidate.GateWeights = map[string]int{
		"syntax_gate":    50,
		"execution_gate": 50,
	}

	inputs := []policy.GateInput{
		{
			Gates: []policy.GateResult{
				{Name: "syntax_gate", Passed: true},
				{Name: "execution_gate", Passed: false},
			},
			Cert:  &policy.CertificationResult{Deterministic: true},
			Chaos: &policy.ChaosReport{Active: false},
		},
	}

	v := NewValidator(engineFactory)
	currentEngine := engineFactory(current)

	proposal, err := v.Validate(current, candidate, inputs, currentEngine)
	if err != nil {
		t.Fatal(err)
	}

	if !proposal.Accepted {
		t.Fatalf("expected proposal to be accepted, got rejected: %+v", proposal)
	}
	if proposal.CandidateScore <= proposal.CurrentScore {
		t.Fatalf("expected candidate score > current score, got %.2f <= %.2f", proposal.CandidateScore, proposal.CurrentScore)
	}
}

func TestValidator_RejectsLowConfidence(t *testing.T) {
	current := policy.DefaultTunedConfig()
	candidate := current
	candidate.ThresholdPass = 89

	inputs := []policy.GateInput{
		{
			Gates: []policy.GateResult{
				{Name: "syntax_gate", Passed: true},
			},
			Cert:  &policy.CertificationResult{Deterministic: true},
			Chaos: &policy.ChaosReport{Active: false},
		},
	}

	v := NewValidator(engineFactory)
	currentEngine := engineFactory(current)

	proposal, err := v.Validate(current, candidate, inputs, currentEngine)
	if err != nil {
		t.Fatal(err)
	}

	if proposal.Accepted {
		t.Fatalf("expected proposal to be rejected, got accepted: %+v", proposal)
	}
}

func TestValidator_CacheHit(t *testing.T) {
	current := policy.DefaultTunedConfig()
	candidate := current
	candidate.ThresholdWarn = 65

	inputs := []policy.GateInput{
		{
			Gates: []policy.GateResult{
				{Name: "syntax_gate", Passed: true},
			},
			Cert:  &policy.CertificationResult{Deterministic: true},
			Chaos: &policy.ChaosReport{Active: false},
		},
	}

	v := NewValidator(engineFactory)
	currentEngine := engineFactory(current)

	p1, err := v.Validate(current, candidate, inputs, currentEngine)
	if err != nil {
		t.Fatal(err)
	}

	p2, err := v.Validate(current, candidate, inputs, currentEngine)
	if err != nil {
		t.Fatal(err)
	}

	if p1.ID != p2.ID || p1.Accepted != p2.Accepted || p1.Reason != p2.Reason {
		t.Fatalf("expected cached result to match exactly")
	}
}
