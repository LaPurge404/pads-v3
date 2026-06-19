package evolution

// Selector is the interface for any bandit-like selection strategy.
type Selector interface {
	AddArm(name string)
	Update(name string, reward float64)
	Select() string
	ListArms() []string
}
