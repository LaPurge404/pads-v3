package learner

import "pads-v3/internal/policy"

// TunedConfig is an alias to the single source of truth in the policy package.
type TunedConfig = policy.TunedConfig

// LearningReport explains the adjustments made.
type LearningReport struct {
    Adjustments  []Adjustment
    AnomalyScore float64
    Confidence   float64
}

// Adjustment describes a single configuration change.
type Adjustment struct {
    Parameter string
    Target    string
    OldValue  float64
    NewValue  float64
    Reason    string
}

func (r *LearningReport) Anomaly() float64 {
    return r.AnomalyScore
}

// LearnerSnapshot is a serializable snapshot of the learner state.
type LearnerSnapshot struct {
    EMAScore     float64
    BlockRateEMA float64
    ScoreMean    float64
    ScoreStdDev  float64
    EventCount   int
}
