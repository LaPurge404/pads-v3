package evolution

import "fmt"

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

func (l *SafeEvolutionLoopV3) Evolve(candidate Candidate, current Candidate, weight float64) (CycleResult, bool, error) {
    result, accepted := l.orchestrator.Evaluate(candidate, current, weight)

    l.rollback.wal.Append(candidate.Score, current.Score, weight, l.mode)
    l.detector.Add(float64(result.Score))

    if _, rolledBack := l.rollback.RollbackIfUnstable(); rolledBack {
        return result, false, fmt.Errorf("rollback déclenché")
    }

    l.sequence++

    // Calcul du score de stabilité (moyenne de la fenêtre du détecteur)
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
    }
    if err := l.eventStore.Append(ev); err != nil {
        return result, accepted, fmt.Errorf("échec écriture event store : %w", err)
    }

    return result, accepted, nil
}

// StabilityScore retourne la moyenne actuelle de la fenêtre du détecteur.
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
