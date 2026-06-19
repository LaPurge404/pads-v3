package evolution

// Rewarder computes the reward associated with an evolution cycle.
// oldStability: stability score before evolution
// newStability: stability score after evolution
// accepted: whether the candidate was accepted by the orchestrator
type Rewarder interface {
	ComputeReward(oldStability, newStability float64, accepted bool) float64
}

// DeltaRewarder is a simple Rewarder: reward = delta(stability).
// An improvement yields a positive reward, a degradation yields a penalty.
type DeltaRewarder struct{}

func (d DeltaRewarder) ComputeReward(oldStability, newStability float64, accepted bool) float64 {
	delta := newStability - oldStability
	// If the candidate is rejected despite an improvement, no reward is given
	// (because the final decision was negative).
	if !accepted {
		return 0
	}
	return delta
}
