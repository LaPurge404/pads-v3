package evolution

type Orchestrator struct {
	Evaluator *MultiCycleEvaluator
	Gate      *StabilityGate
}

func NewOrchestrator(e *MultiCycleEvaluator, g *StabilityGate) *Orchestrator {
	return &Orchestrator{
		Evaluator: e,
		Gate:      g,
	}
}

func (o *Orchestrator) Evaluate(a Candidate, b Candidate, weight float64) (CycleResult, bool) {
	result := o.Evaluator.Evaluate(a, b, weight)

	gateOk := o.Gate.Check(result.Score)
	// Final acceptance: evaluator AND stability gate
	finalOk := result.Accepted && gateOk

	return result, finalOk
}
