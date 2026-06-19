package evolution

type ReplayEngine struct {
	events []Event
}

func NewReplayEngine(events []Event) *ReplayEngine {
	return &ReplayEngine{events: events}
}

func (r *ReplayEngine) Rebuild() SystemState {
	state := SystemState{
		Bandit: BanditState{
			Arms: make(map[string]float64),
			Seed: 0,
		},
		Gate: GateState{
			LongWindow:     make([]float64, 0),
			DetectorWindow: make([]float64, 0),
			Threshold:      0.5,
			VarianceThresh: 10.0,
			AdaptiveFactor: 2.0,
			MaxWindow:      10,
		},
		DetectorWindow: make([]float64, 0),
		Mode:           ModeStable,
	}

	bandit := NewBanditWithSeed(0)
	detector := NewAntiCollapseDetector(5, 10.0)
	gate := NewStabilityGate()

	for _, ev := range r.events {
		state.Mode = ev.Mode
		state.Sequence = ev.Sequence

		if ev.BanditSeed != 0 {
			bandit = NewBanditWithSeed(ev.BanditSeed)
		}

		detector.Add(float64(ev.CandidateScore))
		gate.longWindow = append(gate.longWindow, float64(ev.CandidateScore))
		if len(gate.longWindow) > gate.maxWindow {
			gate.longWindow = gate.longWindow[1:]
		}

		if ev.GateVariance > 0 || ev.GateThreshold > 0 {
			gateState := GateState{
				LongWindow:     copySlice(gate.longWindow),
				DetectorWindow: copySlice(detector.window),
				Threshold:      ev.GateThreshold,
				VarianceThresh: 10.0,
				AdaptiveFactor: 2.0,
				MaxWindow:      10,
			}
			gate.ImportState(gateState)
		}

		state.Bandit.Arms = copyArms(bandit.arms)
		state.Bandit.Seed = ev.BanditSeed
		state.Gate = gate.ExportState()
		state.DetectorWindow = copySlice(detector.window)

		// Calcul du score de stabilité = moyenne de la fenêtre du détecteur
		if len(state.DetectorWindow) > 0 {
			sum := 0.0
			for _, v := range state.DetectorWindow {
				sum += v
			}
			state.StabilityScore = sum / float64(len(state.DetectorWindow))
		}
	}

	return state
}

func copySlice(src []float64) []float64 {
	dst := make([]float64, len(src))
	copy(dst, src)
	return dst
}

func copyArms(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
