package evolution

import "testing"

func TestController_Decide_AcceptsBetterCandidate(t *testing.T) {
eval := &MultiCycleEvaluator{}
gate := NewStabilityGate()
o := NewOrchestrator(eval, gate)

result, ok := o.Evaluate(
Candidate{Score: 100},
Candidate{Score: 50},
0.9,
)

if !ok {
t.Fatalf("expected accept=true, got false")
}
if !result.Accepted {
t.Fatalf("expected result.Accepted=true, got false")
}
}

func TestController_Decide_RejectsLowConfidence(t *testing.T) {
eval := &MultiCycleEvaluator{}
gate := NewStabilityGate()
o := NewOrchestrator(eval, gate)

result, ok := o.Evaluate(
Candidate{Score: 10},
Candidate{Score: 100},
0.1,
)

if ok {
t.Fatalf("expected reject, got accept")
}
if result.Accepted {
t.Fatalf("expected result.Accepted=false, got true")
}
}

func TestOrchestrator_Evaluate_ReturnsConsistentTuple(t *testing.T) {
eval := &MultiCycleEvaluator{}
gate := NewStabilityGate()
o := NewOrchestrator(eval, gate)

r1, ok1 := o.Evaluate(Candidate{Score: 10}, Candidate{Score: 10}, 0.5)
r2, ok2 := o.Evaluate(Candidate{Score: 10}, Candidate{Score: 10}, 0.5)

if ok1 != ok2 {
t.Fatalf("inconsistent acceptance: %v vs %v", ok1, ok2)
}
if r1.Accepted != r2.Accepted || r1.Score != r2.Score {
t.Fatalf("inconsistent result tuple")
}
}

func TestBandit_BasicLearning(t *testing.T) {
b := NewBandit()
b.AddArm("A")
b.AddArm("B")

best := b.Select()
if best == "" {
t.Fatalf("expected a selected arm")
}

b.Update(best, 1.0)
}
