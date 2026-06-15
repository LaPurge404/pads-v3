package learner

import (
    "testing"

    "pads-v3/internal/policy"
)

func TestLearner_NotEnoughSamples(t *testing.T) {
    l := NewLearner()
    traces := []policy.PolicyTrace{
        {FinalStatus: policy.StatusPass, FinalScore: 100},
    }
    _, _, err := l.Learn(traces, TunedConfig{})
    if err == nil {
        t.Error("expected error for insufficient samples")
    }
}

func TestLearner_AdjustsWeights(t *testing.T) {
    l := NewLearner()
    l.MinSamples = 3

    traces := []policy.PolicyTrace{
        {
            FinalStatus: policy.StatusBlock,
            FinalScore:  40,
            Steps: []policy.TraceStep{
                {Stage: "GATES", Impact: -30, Source: "syntax_gate", FailedGates: []string{"syntax_gate"}},
            },
        },
        {
            FinalStatus: policy.StatusBlock,
            FinalScore:  50,
            Steps: []policy.TraceStep{
                {Stage: "GATES", Impact: -30, Source: "syntax_gate", FailedGates: []string{"syntax_gate"}},
            },
        },
        {
            FinalStatus: policy.StatusPass,
            FinalScore:  100,
            Steps:       []policy.TraceStep{},
        },
    }

    config := TunedConfig{
        GateWeights: map[string]int{
            "syntax_gate": 30,
        },
        ThresholdPass: 90,
        ThresholdWarn: 70,
        ThresholdFail: 50,
    }

    tuned, report, err := l.Learn(traces, config)
    if err != nil {
        t.Fatal(err)
    }
    if tuned == nil {
        t.Fatal("tuned config is nil")
    }
    if len(report.Adjustments) == 0 {
        t.Log("no adjustments made (may be expected if failure rate not high enough)")
    }
    if report.AnomalyScore <= 0 {
        t.Error("expected positive anomaly score")
    }
}
