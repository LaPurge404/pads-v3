package evolution

type Gate struct {
MinImprovement float64
MinConfidence  float64
}

func NewGate() *Gate {
return &Gate{
MinImprovement: 0.05,
MinConfidence:  0.4,
}
}

func (g *Gate) Allow(candidateScore, currentScore, confidence float64) bool {

improvement := candidateScore - currentScore

if confidence < g.MinConfidence {
return false
}

if improvement < g.MinImprovement {
return false
}

return true
}
