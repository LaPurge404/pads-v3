package change

type PolicyChangeGate struct {
	MinConfidence      float64
	MinAbsoluteImprove float64
}

func NewPolicyChangeGate(conf, delta float64) *PolicyChangeGate {
	return &PolicyChangeGate{
		MinConfidence:      conf,
		MinAbsoluteImprove: delta,
	}
}

func (g *PolicyChangeGate) Evaluate(candidateAvg, currentAvg, confidence float64) bool {
	if currentAvg == 0 {
		return false
	}

	delta := candidateAvg - currentAvg

	if confidence < g.MinConfidence {
		return false
	}

	if delta < g.MinAbsoluteImprove {
		return false
	}

	return true
}
