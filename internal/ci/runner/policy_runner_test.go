package runner

import (
    "testing"

    "pads-v3/internal/ci/certification"
    "pads-v3/internal/ci/gates"
    "pads-v3/internal/event"
    "pads-v3/internal/policy"
    "pads-v3/internal/policy/learner"
    "pads-v3/internal/policy/shadow"
    "pads-v3/internal/policy/wal"
)

func TestPolicyRunner_FullPipeline(t *testing.T) {
    tmpDir := t.TempDir()
    walPath := tmpDir + "/policy.log"

    pw, _ := wal.NewPolicyWAL(walPath)
    store := policy.NewConfigStore(policy.DefaultTunedConfig())
    pe := policy.NewEngine(store)
    ee := &policy.ExplainEngine{}
    lrn := learner.NewLearner()
    lrn.MinSamples = 1

    // Adapter qui convertit TunedConfig en *Engine pour le ShadowEvaluator
    engineFactory := func(cfg policy.TunedConfig) *policy.Engine {
        return policy.NewEngine(policy.NewConfigStore(cfg))
    }

    runner := &PolicyRunner{
        GateRunner: &GateRunner{
            Gates: []gates.Gate{
                &gates.SyntaxGate{},
            },
        },
        PolicyEngine:    pe,
        ExplainEngine:   ee,
        PolicyWAL:       pw,
        Learner:         lrn,
        ConfigStore:     store,
        ShadowEvaluator: shadow.New(engineFactory),
    }

    input := gates.GateInput{
        Events: []event.CanonicalEvent{
            {Type: "CI_JOB_STARTED", JobID: "test"},
        },
    }
    cert := &certification.Certificate{
        Deterministic: true,
        WALHash:       "abc123",
        ReplayHash:    "abc123",
    }
    chaosEvents := []string{}

    decision, err := runner.Run(input, cert, chaosEvents)
    if err != nil {
        t.Fatal(err)
    }
    if decision.Status != policy.StatusPass {
        t.Errorf("expected PASS, got %s (score=%.1f)", decision.Status, decision.Score)
    }

    events, _ := pw.ReadAll()
    if len(events) != 1 {
        t.Errorf("expected 1 WAL event, got %d", len(events))
    }

    if len(runner.getRecentInputs()) != 1 {
        t.Errorf("expected 1 buffered input, got %d", len(runner.getRecentInputs()))
    }

    t.Logf("Decision: status=%s score=%.1f", decision.Status, decision.Score)
}
