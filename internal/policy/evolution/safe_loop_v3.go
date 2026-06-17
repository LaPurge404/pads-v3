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
        return result, false, fmt.Errorf("[evolution] [ERR_ROLLBACK_TRIGGERED] rollback déclenché for candidate score %d", candidate.Score)
    }

    l.sequence++

    var stabilityScore float64
    if len(l.detector.window) > 0 {
        sum := 0.0
        for _, v := range l.detector.window {
            sum += v
        }
        stabilityScore = sum / float64(len(l.detector.window))
    }

    // Tendance sur la fenêtre du détecteur
    trend := 0.0
    if len(l.detector.window) >= 2 {
        trend = linearSlope(l.detector.window)
    }

    // Construction de la raison
    trendText := TrendDescription(trend)
    var reason string
    if accepted {
        reason = fmt.Sprintf("✅ Accepté — Qualité : %.0f%% (précédent : %d). Tendance : %s.",
            stabilityScore, current.Score, trendText)
    } else {
        reason = fmt.Sprintf("❌ Rejeté — Qualité : %.0f%% (précédent : %d). Tendance : %s.",
            stabilityScore, current.Score, trendText)
    }

    ev := Event{
        Sequence:       l.sequence,
        CandidateScore: candidate.Score,
        CurrentScore:   current.Score,
        Weight:         weight,
        Mode:           l.mode,
        BanditSeed:     l.currentSeed,
        StabilityScore: stabilityScore,
        Trend:          trend,
        Reason:         reason,
    }
    if err := l.eventStore.Append(ev); err != nil {
        return result, accepted, fmt.Errorf("[evolution] [ERR_EVENT_STORE_WRITE_FAILED] échec écriture event store for sequence %d: %w", l.sequence, err)
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

// trendDescription convertit une pente en mot lisible.
func TrendDescription(slope float64) string {
    switch {
    case slope > 1.0:
        return "en forte hausse 📈"
    case slope > 0.1:
        return "en légère hausse ↗️"
    case slope < -1.0:
        return "en forte baisse 📉"
    case slope < -0.1:
        return "en légère baisse ↘️"
    default:
        return "stable ➡️"
    }
}
