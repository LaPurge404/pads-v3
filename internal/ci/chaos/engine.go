package chaos

import "math/rand"

// Engine injects faults during CI execution.
type Engine struct {
	Mode   Mode
	Faults []Fault
}

// Inject randomly applies a fault depending on the current mode.
func (e *Engine) Inject(ctx *Context) error {
	if len(e.Faults) == 0 {
		return nil
	}

	switch e.Mode {
	case ModeSilent:
		if rand.Float32() < 0.2 {
			return e.Faults[0].Apply(ctx)
		}

	case ModeHard:
		if rand.Float32() < 0.5 {
			return e.Faults[rand.Intn(len(e.Faults))].Apply(ctx)
		}

	case ModeFull:
		return e.Faults[rand.Intn(len(e.Faults))].Apply(ctx)
	}

	return nil
}
