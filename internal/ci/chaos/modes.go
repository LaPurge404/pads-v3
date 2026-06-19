package chaos

// Mode defines the intensity of chaos injection.
type Mode int

const (
	ModeSilent Mode = iota // small delays, non-breaking
	ModeHard               // injected failures
	ModeFull               // corruption + crashes
)
