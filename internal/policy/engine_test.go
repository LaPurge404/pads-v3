package policy

import (
    "testing"
)

func TestPolicyEngine_AllPass(t *testing.T) {
    engine := NewEngine(nil) // nil crée un store par défaut
    gates := []GateResult{
        {Name: "syntax_gate", Passed: true},
        {Name: "semantic_gate", Passed: true},
        {Name: "execution_gate", Passed: true},
        {Name: "determinism_gate", Passed: true},
    }
    cert := &CertificationResult{Deterministic: true}
    chaos := &ChaosReport{Active: false}

    decision := engine.Evaluate(gates, cert, chaos)
    if decision.Status != StatusPass {
        t.Errorf("expected PASS, got %s (score=%.1f)", decision.Status, decision.Score)
    }
    if decision.Score != 100.0 {
        t.Errorf("expected score 100, got %.1f", decision.Score)
    }
}

func TestPolicyEngine_HardFailGateCaps(t *testing.T) {
    engine := NewEngine(nil)
    gates := []GateResult{
        {Name: "syntax_gate", Passed: true},
        {Name: "semantic_gate", Passed: true},
        {Name: "execution_gate", Passed: false},
        {Name: "determinism_gate", Passed: true},
    }
    cert := &CertificationResult{Deterministic: true}
    chaos := &ChaosReport{Active: false}

    decision := engine.Evaluate(gates, cert, chaos)
    if decision.Score > 60 {
        t.Errorf("expected score <= 60 after hard-fail, got %.1f", decision.Score)
    }
}

func TestPolicyEngine_NonDeterministicBlock(t *testing.T) {
    engine := NewEngine(nil)
    gates := []GateResult{
        {Name: "syntax_gate", Passed: true},
    }
    cert := &CertificationResult{Deterministic: false}
    chaos := &ChaosReport{Active: false}

    decision := engine.Evaluate(gates, cert, chaos)
    if decision.Status != StatusBlock {
        t.Errorf("expected BLOCK for non-deterministic, got %s", decision.Status)
    }
}

func TestPolicyEngine_ChaosPenalty(t *testing.T) {
    engine := NewEngine(nil)
    gates := []GateResult{
        {Name: "syntax_gate", Passed: true},
        {Name: "semantic_gate", Passed: true},
        {Name: "execution_gate", Passed: true},
        {Name: "determinism_gate", Passed: true},
    }
    cert := &CertificationResult{Deterministic: true}
    chaos := &ChaosReport{Active: true, Mode: "Full"}

    decision := engine.Evaluate(gates, cert, chaos)
    if decision.Score != 70.0 {
        t.Errorf("expected score 70 after Full chaos, got %.1f", decision.Score)
    }
}

func TestPolicyEngine_ScoreClamp(t *testing.T) {
    engine := NewEngine(nil)
    gates := []GateResult{
        {Name: "syntax_gate", Passed: false},
        {Name: "semantic_gate", Passed: false},
        {Name: "execution_gate", Passed: false},
        {Name: "determinism_gate", Passed: false},
    }
    cert := &CertificationResult{Deterministic: true}
    chaos := &ChaosReport{Active: true, Mode: "Full"}
    decision := engine.Evaluate(gates, cert, chaos)
    if decision.Score != 0.0 {
        t.Errorf("expected score 0 after clamp, got %.1f", decision.Score)
    }
}
