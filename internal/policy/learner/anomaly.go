package learner

import (
    "math"
    "sync"

    "pads-v3/internal/policy/wal"
)

// AnomalyDetector uses Exponential Moving Average (EMA) and Z-score
// to detect drift and spikes in policy decision scores.
// It is safe for concurrent use.
type AnomalyDetector struct {
    mu             sync.Mutex
    alpha          float64
    emaScore       float64
    emaBlockRate   float64
    emaChaosRate   float64
    scoreMean      float64
    scoreStdDev    float64
    count          int
    zScoreThreshold  float64
    emaDropThreshold float64
}

func NewAnomalyDetector() *AnomalyDetector {
    return &AnomalyDetector{
        alpha:            0.2,
        zScoreThreshold:  2.0,
        emaDropThreshold: 20.0,
    }
}

// Replay rebuilds the detector state from a complete history of policy events.
func (d *AnomalyDetector) Replay(events []wal.PolicyEvent) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.emaScore = 0
    d.emaBlockRate = 0
    d.emaChaosRate = 0
    d.scoreMean = 0
    d.scoreStdDev = 0
    d.count = 0

    for _, e := range events {
        chaosActive := false
        for _, step := range e.Trace.Steps {
            if step.Stage == "CHAOS" {
                chaosActive = true
                break
            }
        }
        d.updateEMA(e.Score, string(e.Trace.FinalStatus), chaosActive)
    }
}

// UpdateEMA updates the exponential moving averages (must be called with lock held).
func (d *AnomalyDetector) updateEMA(score float64, status string, chaosActive bool) {
    if d.count == 0 {
        d.emaScore = score
        d.scoreMean = score
        d.scoreStdDev = 0
        d.count = 1
        return
    }

    d.emaScore = d.alpha*score + (1-d.alpha)*d.emaScore

    blocked := 0.0
    if status == "BLOCK" || status == "FAIL" {
        blocked = 1.0
    }
    if d.count == 0 {
        d.emaBlockRate = blocked
    } else {
        d.emaBlockRate = d.alpha*blocked + (1-d.alpha)*d.emaBlockRate
    }

    chaosVal := 0.0
    if chaosActive {
        chaosVal = 1.0
    }
    if d.count == 0 {
        d.emaChaosRate = chaosVal
    } else {
        d.emaChaosRate = d.alpha*chaosVal + (1-d.alpha)*d.emaChaosRate
    }

    d.count++
    delta := score - d.scoreMean
    d.scoreMean += delta / float64(d.count)
    delta2 := score - d.scoreMean
    d.scoreStdDev = math.Sqrt((float64(d.count-1)*d.scoreStdDev*d.scoreStdDev + delta*delta2) / float64(d.count))
}

// IsAnomaly checks if the current score is anomalous.
func (d *AnomalyDetector) IsAnomaly(score float64) (bool, string) {
    d.mu.Lock()
    defer d.mu.Unlock()

    if d.count < 3 {
        return false, "not enough data"
    }

    reasons := []string{}
    if d.scoreStdDev > 0 {
        zScore := (score - d.scoreMean) / d.scoreStdDev
        if math.Abs(zScore) > d.zScoreThreshold {
            reasons = append(reasons, "z-score anomaly detected")
        }
    }
    if d.emaScore-score > d.emaDropThreshold {
        reasons = append(reasons, "EMA score drop detected")
    }
    return len(reasons) > 0, joinStrings(reasons, "; ")
}

// EMA returns the current EMA of the score.
func (d *AnomalyDetector) EMA() float64 {
    d.mu.Lock()
    defer d.mu.Unlock()
    return d.emaScore
}

// BlockRateEMA returns the EMA of the block rate.
func (d *AnomalyDetector) BlockRateEMA() float64 {
    d.mu.Lock()
    defer d.mu.Unlock()
    return d.emaBlockRate
}

// Snapshot returns a serializable snapshot of the detector state.
func (d *AnomalyDetector) Snapshot() LearnerSnapshot {
    d.mu.Lock()
    defer d.mu.Unlock()
    return LearnerSnapshot{
        EMAScore:     d.emaScore,
        BlockRateEMA: d.emaBlockRate,
        ScoreMean:    d.scoreMean,
        ScoreStdDev:  d.scoreStdDev,
        EventCount:   d.count,
    }
}

func joinStrings(strs []string, sep string) string {
    result := ""
    for i, s := range strs {
        if i > 0 {
            result += sep
        }
        result += s
    }
    return result
}

// UpdateEMA is the public wrapper for updating internal EMA state.
// Used by tests and external callers.
func (d *AnomalyDetector) UpdateEMA(score float64, status string, chaosActive bool) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.updateEMA(score, status, chaosActive)
}
