package evolution

type MultiCycleEngine struct {
Evaluator *MultiCycleEvaluator
}

func NewMultiCycleEngine(e *MultiCycleEvaluator) *MultiCycleEngine {
return &MultiCycleEngine{Evaluator: e}
}

func (m *MultiCycleEngine) Run(c Candidate, current Candidate, confidence float64) CycleResult {
return m.Evaluator.Evaluate(c, current, confidence)
}
