package evolution_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"pads-v3/internal/policy/evolution"
)

// TestSafeEvolutionLoopV3_ConcurrentEvolveAndRead is a `-race` regression test
// for the data race that previously existed on SafeEvolutionLoopV3.sequence
// and AntiCollapseDetector.window. With sync.RWMutex on the loop, running
// this test with `go test -race ./internal/policy/evolution/ -run
// TestSafeEvolutionLoopV3_ConcurrentEvolveAndRead -count=10` must report NO
// data race. Before the fix it crashed under -race within the first iteration.
func TestSafeEvolutionLoopV3_ConcurrentEvolveAndRead(t *testing.T) {
	t.Parallel()

	orch := evolution.NewOrchestrator(
		evolution.NewMultiCycleEvaluator(),
		evolution.NewStabilityGate(),
	)
	es := evolution.NewEventStore(t.TempDir() + "/race_ev.log")
	wal := evolution.NewWAL("")
	detector := evolution.NewAntiCollapseDetector(8, 100.0) // very high threshold so nothing rolls back
	bandit := evolution.NewBandit()
	loop := evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, bandit)

	// Candidate.Score is int (see internal/policy/evolution/types.go), so we
	// vary by id+i%5 as an int. The float64 form (50 + float64(id + i % 5))
	// does not compile and was caught by go vet/test on Go 1.26.x.
	const (
		writers        = 4
		readers        = 8
		evolutionsPer  = 25
		stabilityReads = 1000
	)

	var wg sync.WaitGroup
	var acceptedOK atomic.Int64

	// Writers: hammer Evolve concurrently. Previously raced on sequence++ and
	// detector.window append. RWMutex write-lock serializes them.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < evolutionsPer; i++ {
				ok, err := loop.Evolve(
					evolution.Candidate{Score: 50 + (id + i%5)},
					evolution.Candidate{Score: 50},
					1.0,
				)
				if err == nil && ok {
					acceptedOK.Add(1)
				}
			}
		}(w)
	}

	// Readers: hammer StabilityScore via RLock. Must be safe to run while
	// writers are appending to the detector window concurrently.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < stabilityReads; i++ {
				_ = loop.StabilityScore()
			}
		}()
	}

	wg.Wait()

	if got := acceptedOK.Load(); got == 0 {
		t.Fatalf("expected at least one accepted evolution; got %d", got)
	}
}
