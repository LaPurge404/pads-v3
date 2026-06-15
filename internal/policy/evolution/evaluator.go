package evolution

type MultiCycleEvaluator struct{}

func NewMultiCycleEvaluator() *MultiCycleEvaluator {
return &MultiCycleEvaluator{}
}

func (m *MultiCycleEvaluator) Evaluate(a Candidate, b Candidate, weight float64) CycleResult {

scoreA := int(float64(a.Score) * weight)
scoreB := int(float64(b.Score) * weight)

accepted := scoreA >= scoreB

return CycleResult{
Score:    scoreA,
Accepted: accepted,
}
}
