package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestSafeEvolutionLoopV3_Evolve(t *testing.T) {
    orch := evolution.NewOrchestrator(
        evolution.NewMultiCycleEvaluator(),
        evolution.NewStabilityGate(),
    )
    es := evolution.NewEventStore(t.TempDir() + "/ev.log")
    wal := evolution.NewWAL()
    detector := evolution.NewAntiCollapseDetector(5, 10.0)
    bandit := evolution.NewBandit()

    loop := evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, bandit)

    result, accepted, err := loop.Evolve(
        evolution.Candidate{Score: 100},
        evolution.Candidate{Score: 50},
        1.0,
    )
    if err != nil {
        t.Fatal(err)
    }
    if !accepted || result.Score != 100 {
        t.Fatalf("unexpected result: %+v, accepted=%v", result, accepted)
    }

    // Vérifier que l'événement a été stocké
    events, _ := es.LoadAll()
    if len(events) == 0 {
        t.Fatal("no event stored")
    }
}
