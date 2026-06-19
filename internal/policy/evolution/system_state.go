package evolution

type BanditState struct {
	Arms map[string]float64
	Seed int64
}

type GateState struct {
	LongWindow     []float64
	DetectorWindow []float64
	Threshold      float64
	VarianceThresh float64
	AdaptiveFactor float64
	MaxWindow      int
}

// SystemState représente l'état complet à un instant T.
type SystemState struct {
	Bandit         BanditState
	Gate           GateState
	DetectorWindow []float64
	Mode           Mode
	Sequence       int
	StabilityScore float64 // score de stabilité calculé après le dernier cycle
}
