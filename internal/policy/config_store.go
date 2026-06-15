package policy

import (
    "math"
    "sync"
)

// ConfigStore holds the active TunedConfig used by the PolicyEngine.
// It applies adaptive EMA smoothing, drift detection, and rollback.
type ConfigStore struct {
    mu       sync.RWMutex
    config   TunedConfig
    history  []TunedConfig
    emaAlpha float64 // base smoothing factor (0 = no smoothing, 1 = instant)

    // Drift detection state
    scoreVariance float64 // running variance of recent scores
    scoreMean     float64 // running mean of recent scores
    scoreCount    int
    instability   float64 // 0 = stable, 1 = highly unstable
}

// NewConfigStore initializes the config store with a baseline configuration.
func NewConfigStore(initial TunedConfig) *ConfigStore {
    return &ConfigStore{
        config:   initial,
        history:  nil,
        emaAlpha: 0.3,
    }
}

// Get returns a copy of the current configuration.
func (s *ConfigStore) Get() TunedConfig {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.config
}

// UpdateScore feeds the latest policy score into the drift detector.
// This should be called after each policy decision, before Update.
func (s *ConfigStore) UpdateScore(score float64) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.scoreCount++
    if s.scoreCount == 1 {
        s.scoreMean = score
        return
    }
    delta := score - s.scoreMean
    s.scoreMean += delta / float64(s.scoreCount)
    delta2 := score - s.scoreMean
    s.scoreVariance = (float64(s.scoreCount-1)*s.scoreVariance + delta*delta2) / float64(s.scoreCount)

    // Compute instability from score variance (0-1 scale)
    if s.scoreCount >= 5 {
        cv := math.Sqrt(s.scoreVariance) / s.scoreMean // coefficient of variation
        s.instability = math.Min(1.0, cv*10)            // scale up
    }
}

// ShouldUpdate returns true if the current instability is low enough
// to accept a new configuration.
func (s *ConfigStore) ShouldUpdate() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // Require at least 5 scores before allowing updates
    if s.scoreCount < 5 {
        return false
    }
    // Reject updates if instability > 0.5
    return s.instability <= 0.5
}

// AdaptiveAlpha returns the EMA smoothing factor adjusted for current instability.
func (s *ConfigStore) AdaptiveAlpha() float64 {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // When unstable, increase smoothing (reduce alpha)
    if s.instability > 0.5 {
        return s.emaAlpha * 0.3 // much slower adaptation
    }
    return s.emaAlpha
}

// Update replaces the current configuration after applying adaptive EMA smoothing.
// It also saves the previous configuration in the history for rollback.
func (s *ConfigStore) Update(cfg TunedConfig) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Save current state to history BEFORE modifying
    s.history = append(s.history, s.config)
    if len(s.history) > 10 {
        s.history = s.history[len(s.history)-10:]
    }

    // Compute adaptive alpha based on current instability
    alpha := s.emaAlpha
    if s.instability > 0.5 {
        alpha = s.emaAlpha * 0.3
    }

    // Apply EMA smoothing to numeric fields
    s.config.ThresholdPass = ema(s.config.ThresholdPass, cfg.ThresholdPass, alpha)
    s.config.ThresholdWarn = ema(s.config.ThresholdWarn, cfg.ThresholdWarn, alpha)
    s.config.ThresholdFail = ema(s.config.ThresholdFail, cfg.ThresholdFail, alpha)

    // For maps, we keep the new values directly (they are already conservative)
    if cfg.GateWeights != nil {
        s.config.GateWeights = cfg.GateWeights
    }
    if cfg.ChaosPenalties != nil {
        s.config.ChaosPenalties = cfg.ChaosPenalties
    }
    if cfg.HardFailGates != nil {
        s.config.HardFailGates = cfg.HardFailGates
    }
}

// Rollback restores the configuration to the state before the last update.
func (s *ConfigStore) Rollback() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.history) == 0 {
        return false
    }
    s.config = s.history[len(s.history)-1]
    s.history = s.history[:len(s.history)-1]
    return true
}

func ema(old, new, alpha float64) float64 {
    return alpha*new + (1-alpha)*old
}

// ShadowEvaluate tests a candidate configuration against the current one
// using a set of recent inputs. It returns the candidate score, the current
// score, and whether the candidate should be accepted.
func (s *ConfigStore) ShadowEvaluate(candidate TunedConfig, recentInputs []GateInput, engine *Engine) (candidateAvg, currentAvg float64, accept bool) {
    if len(recentInputs) == 0 {
        return 0, 0, false
    }

    var candidateSum, currentSum float64
    for _, input := range recentInputs {
        // Evaluate with candidate config
        candidateEngine := NewEngine(NewConfigStore(candidate))
        candidateDec := candidateEngine.Evaluate(input.Gates, input.Cert, input.Chaos)
        candidateSum += candidateDec.Score

        // Evaluate with current config
        currentDec := engine.Evaluate(input.Gates, input.Cert, input.Chaos)
        currentSum += currentDec.Score
    }

    n := float64(len(recentInputs))
    candidateAvg = candidateSum / n
    currentAvg = currentSum / n

    // Accept if candidate is better by at least a small epsilon (1%)
    const epsilon = 0.01
    accept = candidateAvg > currentAvg*(1+epsilon)
    return
}
