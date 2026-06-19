package policy

// Engine evaluates CI results and produces a policy decision.
// It reads its configuration from a ConfigStore.
type Engine struct {
	Store *ConfigStore
}

// NewEngine creates a policy engine with sensible defaults stored in the given store.
// If store is nil, a default in-memory store is created (useful for testing).
func NewEngine(store *ConfigStore) *Engine {
	if store == nil {
		store = NewConfigStore(DefaultTunedConfig())
	}
	return &Engine{Store: store}
}

// DefaultTunedConfig returns the baseline configuration.
func DefaultTunedConfig() TunedConfig {
	return TunedConfig{
		GateWeights: map[string]int{
			"syntax_gate":      30,
			"semantic_gate":    20,
			"execution_gate":   25,
			"determinism_gate": 25,
		},
		HardFailGates: map[string]bool{
			"execution_gate":   true,
			"determinism_gate": true,
		},
		ChaosPenalties: map[string]float64{
			"Silent": 5,
			"Hard":   15,
			"Full":   30,
		},
		ThresholdPass: 90,
		ThresholdWarn: 70,
		ThresholdFail: 50,
	}
}

// Evaluate takes all CI inputs and returns a policy decision.
func (e *Engine) Evaluate(gateResults []GateResult, cert *CertificationResult, chaos *ChaosReport) PolicyDecision {
	var reasoning []string
	var actions []Action

	cfg := e.Store.Get()

	totalWeight := 0
	earnedWeight := 0
	hardFailTriggered := false

	for _, g := range gateResults {
		weight, ok := cfg.GateWeights[g.Name]
		if !ok {
			weight = 10
		}
		totalWeight += weight
		if g.Passed {
			earnedWeight += weight
		} else {
			reasoning = append(reasoning, g.Name+" failed")
			if cfg.HardFailGates[g.Name] {
				hardFailTriggered = true
			}
		}
	}

	score := 0.0
	if totalWeight > 0 {
		score = (float64(earnedWeight) / float64(totalWeight)) * 100.0
	}

	if cert != nil && !cert.Deterministic {
		score = 0
		reasoning = append(reasoning, "certification: non-deterministic run")
		return PolicyDecision{
			Status:    StatusBlock,
			Score:     score,
			Actions:   []Action{ActionBlock},
			Reasoning: reasoning,
		}
	}

	if hardFailTriggered {
		if score > 60 {
			score = 60
		}
		reasoning = append(reasoning, "hard-fail gate triggered: score capped at 60")
	}

	if chaos != nil && chaos.Active {
		penalty, ok := cfg.ChaosPenalties[chaos.Mode]
		if !ok {
			penalty = 0
		}
		score -= penalty
		if score < 0 {
			score = 0
		}
		reasoning = append(reasoning, "chaos penalty applied: mode="+chaos.Mode)
	}

	var status Status
	switch {
	case score >= cfg.ThresholdPass:
		status = StatusPass
		actions = append(actions, ActionAllow)
	case score >= cfg.ThresholdWarn:
		status = StatusWarn
		actions = append(actions, ActionWarn)
	case score >= cfg.ThresholdFail:
		status = StatusFail
		actions = append(actions, ActionRetry)
	default:
		status = StatusBlock
		actions = append(actions, ActionBlock, ActionEscalate)
	}

	return PolicyDecision{
		Status:    status,
		Score:     score,
		Actions:   actions,
		Reasoning: reasoning,
	}
}
