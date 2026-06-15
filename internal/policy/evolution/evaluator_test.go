package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestMultiCycleEvaluator_Evaluate(t *testing.T) {
    eval := evolution.NewMultiCycleEvaluator()
    a := evolution.Candidate{Score: 80}
    b := evolution.Candidate{Score: 50}
    res := eval.Evaluate(a, b, 1.0)
    if !res.Accepted {
        t.Fatal("expected 80 >= 50 to be accepted")
    }
    if res.Score != 80 {
        t.Fatalf("expected score 80, got %d", res.Score)
    }
}
