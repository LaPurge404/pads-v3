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
    StabilityScore float64 // score de stabilité après ce cycle
}
