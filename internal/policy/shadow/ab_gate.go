package shadow

// ABGate provides a statistical A/B acceptance criterion.
type ABGate struct {
    MinDelta float64 // minimum absolute improvement
    MinGain  float64 // minimum relative gain (0.005 = 0.5%)
}

// NewABGate creates a gate with the given thresholds.
func NewABGate(minDelta, minGain float64) *ABGate {
    return &ABGate{MinDelta: minDelta, MinGain: minGain}
}

// Accept returns true if the candidate is significantly better than the current.
func (g *ABGate) Accept(candidateAvg, currentAvg float64) bool {
    delta := candidateAvg - currentAvg
    if currentAvg == 0 {
        return false
    }
    gain := delta / currentAvg
    return delta > g.MinDelta && gain > g.MinGain
}
