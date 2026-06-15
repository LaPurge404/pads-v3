package evolution

type Candidate struct {
    Score int
}

type CycleResult struct {
    Score    int
    Accepted bool
}

// Mode utilisé par le Controller
type Mode string

const (
    ModeBandit Mode = "bandit"
    ModeStable Mode = "stable"
    ModeLocked Mode = "locked"
)
