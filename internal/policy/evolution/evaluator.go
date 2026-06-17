package evolution

type MultiCycleEvaluator struct{}

func NewMultiCycleEvaluator() *MultiCycleEvaluator {
return &MultiCycleEvaluator{}
}

func (m *MultiCycleEvaluator) Evaluate(a Candidate, b Candidate, weight float64) CycleResult {
	scoreA := int(float64(a.Score) * weight)
	scoreB := int(float64(b.Score) * weight)

	accepted := scoreA >= scoreB

	// Confidence basée sur l'écart entre les scores normalisés
	// Plus l'écart est grand, plus la confiance est élevée
	var confidence float64
	sum := scoreA + scoreB
	if sum > 0 {
		ratio := float64(scoreA) / float64(sum)
		// Ratio de 0.5 = confiance 0.5 (incertitude max)
		// Ratio de 1.0 = confiance 1.0 (certitude totale)
		confidence = ratio
	} else {
		confidence = 0.5 // par défaut
	}

	return CycleResult{
		Score:      scoreA,
		Confidence: confidence,
		Accepted:   accepted,
	}
}
