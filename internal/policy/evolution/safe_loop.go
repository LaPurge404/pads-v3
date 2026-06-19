package evolution

import "fmt"

type SafeEvolutionLoop struct {
	orchestrator *Orchestrator
	bridge       *WALBridge
	detector     *AntiCollapseDetector
	rollback     *RollbackManager
	mode         Mode
}

func NewSafeEvolutionLoop(o *Orchestrator, bridge *WALBridge, detector *AntiCollapseDetector, mode Mode) *SafeEvolutionLoop {
	// Le rollback manager a besoin du WAL mémoire pour récupérer le dernier snapshot
	return &SafeEvolutionLoop{
		orchestrator: o,
		bridge:       bridge,
		detector:     detector,
		rollback:     NewRollbackManager(bridge.mem, detector),
		mode:         mode,
	}
}

func (l *SafeEvolutionLoop) Evolve(candidate Candidate, current Candidate, weight float64) (CycleResult, bool, error) {
	// 1. Évaluation
	result, accepted := l.orchestrator.Evaluate(candidate, current, weight)

	// 2. Enregistrement WAL (mémoire + disque)
	_, err := l.bridge.Append(candidate.Score, current.Score, weight, l.mode)
	if err != nil {
		return result, accepted, fmt.Errorf("échec d'écriture WAL : %w", err)
	}

	// 3. Surveillance stabilité
	l.detector.Add(float64(result.Score))

	// 4. Rollback automatique si instable
	if entry, rolledBack := l.rollback.RollbackIfUnstable(); rolledBack {
		return result, false, fmt.Errorf("rollback déclenché, dernier état stable : %+v", entry)
	}

	return result, accepted, nil
}
