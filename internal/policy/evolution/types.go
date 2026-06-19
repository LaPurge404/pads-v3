package evolution

type Candidate struct {
	Score int
}

type CycleResult struct {
	Score      int
	Confidence float64 // confidence level in the evaluation (0-1)
	Accepted   bool
}

// Mode used by the Controller
type Mode string

const (
	ModeBandit Mode = "bandit"
	ModeStable Mode = "stable"
	ModeLocked Mode = "locked"
)
