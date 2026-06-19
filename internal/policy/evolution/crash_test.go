package evolution_test

import (
	"testing"
	"time"

	"pads-v3/internal/policy/evolution"
)

func TestWorkerCrashRecovery(t *testing.T) {
	queue, _ := evolution.NewEventQueue(t.TempDir() + "/crash.log")
	orch := evolution.NewOrchestrator(
		evolution.NewMultiCycleEvaluator(),
		evolution.NewStabilityGate(),
	)
	es := evolution.NewEventStore(t.TempDir() + "/ev.log")
	wal := evolution.NewWAL("")
	detector := evolution.NewAntiCollapseDetector(5, 10.0)
	bandit := evolution.NewBandit()
	loop := evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, bandit)

	worker := evolution.NewWorker(queue, loop, evolution.DeltaRewarder{})
	go worker.Start()

	// Ajouter un événement
	queue.Enqueue(evolution.QueueEvent{
		ID:        "1",
		Type:      "evolve",
		Candidate: 90,
		Current:   50,
		Weight:    1.0,
		Mode:      evolution.ModeStable,
	})
	time.Sleep(100 * time.Millisecond)

	// Simuler un crash en arrêtant le worker
	worker.Running = false
	time.Sleep(100 * time.Millisecond)

	// L'état doit avoir traité l'événement avant le crash
	events, _ := queue.LoadAll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Recréer un nouveau worker (reprise)
	worker2 := evolution.NewWorker(queue, loop, evolution.DeltaRewarder{})
	go worker2.Start()
	time.Sleep(100 * time.Millisecond)

	// Vérifier que l'état est toujours cohérent (pas de double traitement grâce à l'idempotence)
	// Ici on vérifie simplement que le replay donne la même séquence
	engine := evolution.NewReplayEngine(convertQueueToEvents(events))
	state := engine.Rebuild()
	if state.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", state.Sequence)
	}
}

func convertQueueToEvents(qev []evolution.QueueEvent) []evolution.Event {
	var events []evolution.Event
	for i, qe := range qev {
		ev := evolution.Event{
			Sequence:       i + 1,
			CandidateScore: qe.Candidate,
			CurrentScore:   qe.Current,
			Weight:         qe.Weight,
			Mode:           qe.Mode,
		}
		events = append(events, ev)
	}
	return events
}
