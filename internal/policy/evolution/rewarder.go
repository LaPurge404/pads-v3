package evolution

// Rewarder calcule la récompense associée à un cycle d'évolution.
// oldStability : score de stabilité avant l'évolution
// newStability : score de stabilité après l'évolution
// accepted     : si le candidat a été accepté par l'orchestrateur
type Rewarder interface {
	ComputeReward(oldStability, newStability float64, accepted bool) float64
}

// DeltaRewarder est un Rewarder simple : récompense = delta(stabilité).
// Une amélioration donne une récompense positive, une dégradation une pénalité.
type DeltaRewarder struct{}

func (d DeltaRewarder) ComputeReward(oldStability, newStability float64, accepted bool) float64 {
	delta := newStability - oldStability
	// Si le candidat est rejeté malgré une amélioration, on ne donne pas de récompense
	// (car la décision finale a été négative).
	if !accepted {
		return 0
	}
	return delta
}
