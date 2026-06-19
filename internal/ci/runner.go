package ci

// Runner is the interface for step execution.
type Runner interface {
	Run(step Step, ctx Context) (StepResult, error)
}
