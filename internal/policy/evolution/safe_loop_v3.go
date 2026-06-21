package evolution

import (
	"fmt"
	"log/slog"
	"sync"
)

type SafeEvolutionLoopV3 struct {
	orchestrator *Orchestrator
	eventStore   *EventStore
	detector     *AntiCollapseDetector
	rollback     *RollbackManager
	mode         Mode
	sequence     int
	selector     Selector
	currentSeed  int64

	// mu protects sequence, mode, currentSeed, and the anti-collapse detector's
	// internal window. Concurrent calls to Evolve() share these fields (the
	// agent pool at internal/agent/pool.go issues parallel evolutions on the
	// same loop instance), so reads/writes must be serialized.
	//
	// mu is a sync.RWMutex to allow many concurrent readers (StabilityScore
	// from /state, dashboards, health checks) while writers (Evolve) hold an
	// exclusive lock. The detector window is also read by orchestrator.Evaluate
	// path through detector.IsStable / IsOscillating; treating those as
	// readers means concurrent StabilityScore reads don't serialize against
	// each other, only against active evolutions.
	//
	// RollbackManager / WAL / EventStore each own their own synchronization
	// and are considered safe to call while holding mu.
	mu sync.RWMutex
}

func NewSafeEvolutionLoopV3(o *Orchestrator, es *EventStore, wal *WAL, detector *AntiCollapseDetector, mode Mode, selector Selector) *SafeEvolutionLoopV3 {
	return &SafeEvolutionLoopV3{
		orchestrator: o,
		eventStore:   es,
		detector:     detector,
		rollback:     NewRollbackManager(wal, detector),
		mode:         mode,
		selector:     selector,
	}
}

// NewSafeEvolutionLoopV3Minimal creates a SafeEvolutionLoopV3 with default dependencies
// suitable for agent pools where shared evolution state is needed but the full
// WAL/event-store infrastructure is not required.
// The loop uses default stability gate (window=10, threshold=0.5) and a
// nil event store (no persistence).
func NewSafeEvolutionLoopV3Minimal(mode Mode, selector Selector) *SafeEvolutionLoopV3 {
	detector := NewAntiCollapseDetector(10, 0.5)
	gate := NewStabilityGateWithDetector(detector)
	evaluator := NewMultiCycleEvaluator()
	orchestrator := NewOrchestrator(evaluator, gate)
	return NewSafeEvolutionLoopV3(orchestrator, nil, nil, detector, mode, selector)
}

func (l *SafeEvolutionLoopV3) Evolve(candidate Candidate, current Candidate, weight float64) (bool, error) {
	// Serialize concurrent evolutions on this loop. The detector's window and
	// the sequence counter are shared state; the agent pool issues parallel
	// Evolve() calls on the same loop instance.
	//
	// Writer lock: any active evolution invalidates the readers' snapshot of
	// the detector window, so Evolve must block ALL concurrent readers until
	// it finishes appending + bumping sequence. RLock is insufficient.
	l.mu.Lock()
	defer l.mu.Unlock()

	result, accepted := l.orchestrator.Evaluate(candidate, current, weight)

	if l.rollback.wal != nil {
		if _, err := l.rollback.wal.Append(candidate.Score, current.Score, weight, l.mode); err != nil {
			slog.Error("WAL Append failed", "err", err)
		}
	}
	l.detector.Add(float64(result.Score))

	if _, rolledBack := l.rollback.RollbackIfUnstable(); rolledBack {
		return false, fmt.Errorf("rollback déclenché")
	}

	l.sequence++

	// Stability score calculation (mean of the detector window)
	var stabilityScore float64
	if len(l.detector.window) > 0 {
		sum := 0.0
		for _, v := range l.detector.window {
			sum += v
		}
		stabilityScore = sum / float64(len(l.detector.window))
	}

	ev := Event{
		Sequence:       l.sequence,
		CandidateScore: candidate.Score,
		CurrentScore:   current.Score,
		Weight:         weight,
		Mode:           l.mode,
		BanditSeed:     l.currentSeed,
		StabilityScore: stabilityScore,
		Reason:         BuildReason(accepted, candidate.Score, current.Score, stabilityScore-float64(current.Score)),
	}
	if l.eventStore != nil {
		if err := l.eventStore.Append(ev); err != nil {
			return accepted, fmt.Errorf("échec écriture event store : %w", err)
		}
	}

	return accepted, nil
}

// StabilityScore returns the current mean of the detector window.
//
// Uses a read-lock so concurrent observability callers (dashboard, /state,
// health) do not block each other; they only contend with active Evolve()
// calls, which hold the exclusive write lock briefly. Safe to call from
// multiple goroutines simultaneously; the underlying detector.window is
// read under RLock and Evolve() publishes new values under the write lock.
func (l *SafeEvolutionLoopV3) StabilityScore() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.stabilityScoreLocked()
}

// stabilityScoreLocked is the unlocked implementation; callers must hold l.mu
// (either RLock for pure reads or Lock for writers).
func (l *SafeEvolutionLoopV3) stabilityScoreLocked() float64 {
	if len(l.detector.window) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range l.detector.window {
		sum += v
	}
	return sum / float64(len(l.detector.window))
}
