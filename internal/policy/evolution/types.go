package evolution

type Candidate struct {
    Score int
}

type CycleResult struct {
	Score      int
	Confidence float64 // niveau de confiance dans l'évaluation (0-1)
	Accepted   bool
}

// Mode utilisé par le Controller
type Mode string

const (
    ModeBandit Mode = "bandit"
    ModeStable Mode = "stable"
    ModeLocked Mode = "locked"
)
