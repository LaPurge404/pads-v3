package policy

import (
	"testing"
)

func TestBuildTrace_AllPass(t *testing.T) {
	engine := &ExplainEngine{}
	gates := []GateResult{
		{Name: "syntax_gate", Passed: true},
		{Name: "semantic_gate", Passed: true},
		{Name: "execution_gate", Passed: true},
		{Name: "determinism_gate", Passed: true},
	}
	cert := &CertificationResult{Deterministic: true}
	chaos := &ChaosReport{Active: false}
	decision := NewEngine(nil).Evaluate(gates, cert, chaos)

	trace := engine.BuildTrace(gates, cert, chaos, decision)
	if trace.FinalStatus != StatusPass {
		t.Errorf("expected PASS trace, got %s", trace.FinalStatus)
	}
	if len(trace.Steps) == 0 {
		t.Error("expected trace steps")
	}
}

func TestBuildTrace_NonDeterministic(t *testing.T) {
	engine := &ExplainEngine{}
	gates := []GateResult{
		{Name: "syntax_gate", Passed: true},
	}
	cert := &CertificationResult{Deterministic: false}
	chaos := &ChaosReport{Active: false}
	decision := NewEngine(nil).Evaluate(gates, cert, chaos)

	trace := engine.BuildTrace(gates, cert, chaos, decision)
	if trace.FinalStatus != StatusBlock {
		t.Errorf("expected BLOCK trace, got %s", trace.FinalStatus)
	}
	hasCertStep := false
	for _, step := range trace.Steps {
		if step.Stage == "CERTIFICATION" {
			hasCertStep = true
		}
	}
	if !hasCertStep {
		t.Error("missing certification step in trace")
	}
}

func TestReplayEngine_Integrity(t *testing.T) {
	engine := &ExplainEngine{}
	gates := []GateResult{
		{Name: "syntax_gate", Passed: true},
		{Name: "execution_gate", Passed: false},
	}
	cert := &CertificationResult{Deterministic: true}
	chaos := &ChaosReport{Active: true, Mode: "Hard"}
	decision := NewEngine(nil).Evaluate(gates, cert, chaos)

	trace := engine.BuildTrace(gates, cert, chaos, decision)
	replay := &ReplayEngine{}
	if !replay.Replay(trace) {
		t.Error("replay integrity check failed")
	}
}

func TestBuildExplanation(t *testing.T) {
	engine := &ExplainEngine{}
	gates := []GateResult{
		{Name: "syntax_gate", Passed: false},
		{Name: "semantic_gate", Passed: true},
		{Name: "execution_gate", Passed: true},
		{Name: "determinism_gate", Passed: true},
	}
	cert := &CertificationResult{Deterministic: true}
	chaos := &ChaosReport{Active: true, Mode: "Silent"}
	decision := NewEngine(nil).Evaluate(gates, cert, chaos)

	trace := engine.BuildTrace(gates, cert, chaos, decision)
	expl := BuildExplanation(trace)

	if expl.Confidence <= 0 {
		t.Errorf("expected positive confidence, got %.2f (score=%.1f)", expl.Confidence, decision.Score)
	}
	if len(expl.KeyReasons) == 0 {
		t.Error("expected key reasons")
	}
	if len(expl.RiskSignals) == 0 {
		t.Error("expected risk signals")
	}
}
