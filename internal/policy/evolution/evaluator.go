package evolution

type MultiCycleEvaluator struct{}

func NewMultiCycleEvaluator() *MultiCycleEvaluator {
	return &MultiCycleEvaluator{}
}

func (m *MultiCycleEvaluator) Evaluate(a Candidate, b Candidate, weight float64) CycleResult {
	scoreA := int(float64(a.Score) * weight)
	scoreB := int(float64(b.Score) * weight)

	accepted := scoreA >= scoreB

	// Confidence based on the gap between normalized scores
	// The larger the gap, the higher the confidence
	var confidence float64
	sum := scoreA + scoreB
	if sum > 0 {
		ratio := float64(scoreA) / float64(sum)
		// Ratio of 0.5 = confidence 0.5 (max uncertainty)
		// Ratio of 1.0 = confidence 1.0 (total certainty)
		confidence = ratio
	} else {
		confidence = 0.5 // default
	}

	return CycleResult{
		Score:      scoreA,
		Confidence: confidence,
		Accepted:   accepted,
	}
}
