package agent

import (
	"testing"
)

func TestMockAgentSolve(t *testing.T) {
	mock := MockAgent{}
	plan, err := mock.Solve(
		Task{Kind: TaskFixBroken, Target: "test.go"},
		Context{FilePath: "test.go"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Kind != ActionLog {
		t.Errorf("expected ActionLog, got %s", plan.Steps[0].Kind)
	}
}
