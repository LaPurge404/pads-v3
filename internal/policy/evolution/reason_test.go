package evolution_test

import (
    "strings"
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestTrendDescription(t *testing.T) {
    tests := []struct {
        slope float64
        want  string
    }{
        {0.0, "stable ➡️"},
        {0.5, "en légère hausse ↗️"},
        {2.0, "en forte hausse 📈"},
        {-0.5, "en légère baisse ↘️"},
        {-2.0, "en forte baisse 📉"},
    }
    for _, tt := range tests {
        got := evolution.TrendDescription(tt.slope)
        if got != tt.want {
            t.Errorf("slope %.1f: got %q, want %q", tt.slope, got, tt.want)
        }
    }
}

func TestConfidenceInResult(t *testing.T) {
    eval := evolution.NewMultiCycleEvaluator()
    res := eval.Evaluate(evolution.Candidate{Score: 100}, evolution.Candidate{Score: 50}, 1.0)
    if res.Confidence < 0.5 {
        t.Errorf("expected confidence >= 0.5, got %.2f", res.Confidence)
    }
    if !res.Accepted {
        t.Error("expected accepted")
    }
}

func TestReasonInEvent(t *testing.T) {
    // Construire une boucle et vérifier que la raison n'est pas vide
    es := evolution.NewEventStore(t.TempDir() + "/ev.log")
    wal := evolution.NewWAL()
    detector := evolution.NewAntiCollapseDetector(5, 10.0)
    selector := evolution.NewUCBSelector(123)
    orch := evolution.NewOrchestrator(evolution.NewMultiCycleEvaluator(), evolution.NewStabilityGate())

    loop := evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, selector)
    _, accepted, err := loop.Evolve(evolution.Candidate{Score: 80}, evolution.Candidate{Score: 50}, 1.0)
    if err != nil {
        t.Fatal(err)
    }
    if !accepted {
        t.Fatal("expected accepted")
    }

    events, _ := es.LoadAll()
    if len(events) == 0 {
        t.Fatal("no event stored")
    }
    if !strings.Contains(events[0].Reason, "✅ Accepté") {
        t.Errorf("reason should contain '✅ Accepté', got %q", events[0].Reason)
    }
}
