package agent

// MockAgent is a simple agent that always returns a fixed plan.
type MockAgent struct{}

func (m MockAgent) Solve(task Task, ctx Context) (Plan, error) {
	// Return a plan that simply logs the task and does nothing.
	return Plan{
		Steps: []Action{
			{
				Kind:    ActionLog,
				Target:  task.Target,
				Command: []string{"echo", "MockAgent: processing task " + string(task.Kind)},
			},
		},
	}, nil
}
