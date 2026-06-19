package evolution

import (
	"fmt"
	"log/slog"

	"pads-v3/internal/metrics"
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
	metrics.EvolutionCycles.Add(1)
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
func (l *SafeEvolutionLoopV3) StabilityScore() float64 {
	if len(l.detector.window) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range l.detector.window {
		sum += v
	}
	return sum / float64(len(l.detector.window))
}
