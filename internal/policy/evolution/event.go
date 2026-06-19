package evolution

type Event struct {
	Sequence       int
	CandidateScore int
	CurrentScore   int
	Weight         float64
	Mode           Mode
	BanditSeed     int64
	GateVariance   float64
	GateThreshold  float64
	StabilityScore float64 // stability score after this cycle
	Reason         string  // human-readable explanation of the decision
}
