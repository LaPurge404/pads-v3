package shadow

import (
    "testing"

    "pads-v3/internal/policy"
)

func TestShadowEvaluator_AcceptBetterCandidate(t *testing.T) {
    factory := func(cfg policy.TunedConfig) *policy.Engine {
        store := policy.NewConfigStore(cfg)
        return policy.NewEngine(store)
    }
    eval := New(factory)

    current := policy.TunedConfig{
        ThresholdPass: 90,
        ThresholdWarn: 70,
        ThresholdFail: 50,
        GateWeights: map[string]int{
            "syntax_gate": 30,
        },
    }
    currentEngine := factory(current)

    candidate := current
    candidate.ThresholdPass = 80

    inputs := []policy.GateInput{
        {
            Gates: []policy.GateResult{
                {Name: "syntax_gate", Passed: true},
            },
            Cert:  &policy.CertificationResult{Deterministic: true},
            Chaos: &policy.ChaosReport{Active: false},
        },
    }

    _, _, accept := eval.Evaluate(candidate, current, inputs, currentEngine)
    if accept {
        t.Log("candidate accepted")
    }
}

func TestShadowEvaluator_CacheHit(t *testing.T) {
    factory := func(cfg policy.TunedConfig) *policy.Engine {
        store := policy.NewConfigStore(cfg)
        return policy.NewEngine(store)
    }
    eval := New(factory)

    cfg := policy.TunedConfig{ThresholdPass: 90}
    engine := factory(cfg)
    inputs := []policy.GateInput{
        {
            Gates: []policy.GateResult{{Name: "syntax_gate", Passed: true}},
            Cert:  &policy.CertificationResult{Deterministic: true},
            Chaos: &policy.ChaosReport{Active: false},
        },
    }

    c1, cu1, _ := eval.Evaluate(cfg, cfg, inputs, engine)
    c2, cu2, _ := eval.Evaluate(cfg, cfg, inputs, engine)

    if c1 != c2 || cu1 != cu2 {
        t.Error("cache should return identical results")
    }
}

func TestShadowEvaluator_EmptyInputs(t *testing.T) {
    factory := func(cfg policy.TunedConfig) *policy.Engine {
        store := policy.NewConfigStore(cfg)
        return policy.NewEngine(store)
    }
    eval := New(factory)
    cfg := policy.TunedConfig{ThresholdPass: 90}
    engine := factory(cfg)

    _, _, accept := eval.Evaluate(cfg, cfg, nil, engine)
    if accept {
        t.Error("should not accept with empty inputs")
    }
}
