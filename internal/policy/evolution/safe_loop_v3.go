package evolution

import "fmt"

type SafeEvolutionLoopV3 struct {
    orchestrator *Orchestrator
    eventStore   *EventStore
    detector     *AntiCollapseDetector
    rollback     *RollbackManager  // garde le lien avec le WAL mémoire pour rollback
    mode         Mode
    sequence     int
    bandit       *Bandit          // bandit contrôlé par seed
    currentSeed  int64
}

func NewSafeEvolutionLoopV3(o *Orchestrator, es *EventStore, wal *WAL, detector *AntiCollapseDetector, mode Mode, bandit *Bandit) *SafeEvolutionLoopV3 {
    return &SafeEvolutionLoopV3{
        orchestrator: o,
        eventStore:   es,
        detector:     detector,
        rollback:     NewRollbackManager(wal, detector),
        mode:         mode,
        bandit:       bandit,
    }
}

// Evolve exécute un cycle complet, enregistre un Event déterministe et persiste.
func (l *SafeEvolutionLoopV3) Evolve(candidate Candidate, current Candidate, weight float64) (CycleResult, bool, error) {
    // 1. Évaluation
    result, accepted := l.orchestrator.Evaluate(candidate, current, weight)

    // 2. Enregistrement WAL mémoire (pour rollback immédiat)
    l.rollback.wal.Append(candidate.Score, current.Score, weight, l.mode)

    // 3. Surveillance stabilité
    l.detector.Add(float64(result.Score))

    // 4. Rollback automatique si instable
    if _, rolledBack := l.rollback.RollbackIfUnstable(); rolledBack {
        return result, false, fmt.Errorf("rollback déclenché")
    }

    // 5. Construction de l'événement déterministe
    l.sequence++
    var banditSeed int64
    if l.bandit != nil {
        // On utilise une seed fixe ou on capture la seed actuelle du bandit
        // Ici on génère une nouvelle seed pour le prochain cycle (simulé)
        banditSeed = l.currentSeed + int64(l.sequence) // simple, déterministe
        l.currentSeed = banditSeed
    }

    ev := Event{
        Sequence:       l.sequence,
        CandidateScore: candidate.Score,
        CurrentScore:   current.Score,
        Weight:         weight,
        Mode:           l.mode,
        BanditSeed:     banditSeed,
    }

    // 6. Persistance de l'événement
    if err := l.eventStore.Append(ev); err != nil {
        return result, accepted, fmt.Errorf("échec écriture event store : %w", err)
    }

    return result, accepted, nil
}
