package evolution

import "math"

type StabilityGate struct {
threshold         float64
varianceThreshold float64
detector          *AntiCollapseDetector // toujours non nil après construction
adaptiveFactor    float64
maxWindow         int
longWindow        []float64
}

// NewStabilityGateWithDetector crée un gate avec un détecteur injecté (obligatoire).
func NewStabilityGateWithDetector(detector *AntiCollapseDetector) *StabilityGate {
return &StabilityGate{
threshold:         0.5,
varianceThreshold: 10.0,
detector:          detector,
adaptiveFactor:    2.0,
maxWindow:         10,
longWindow:        make([]float64, 0, 10),
}
}

// NewStabilityGate crée un gate avec un détecteur par défaut (usage standard).
func NewStabilityGate() *StabilityGate {
return NewStabilityGateWithDetector(NewAntiCollapseDetector(5, 10.0))
}

// NewStabilityGateV2 permet de paramétrer le gate.
func NewStabilityGateV2(threshold float64, windowSize int, varianceThreshold float64) *StabilityGate {
detector := NewAntiCollapseDetector(windowSize, varianceThreshold)
return &StabilityGate{
threshold:         threshold,
varianceThreshold: varianceThreshold,
detector:          detector,
adaptiveFactor:    2.0,
maxWindow:         windowSize,
longWindow:        make([]float64, 0, windowSize),
}
}

func (g *StabilityGate) Check(score int) bool {
g.detector.Add(float64(score))
g.longWindow = append(g.longWindow, float64(score))
if len(g.longWindow) > g.maxWindow {
g.longWindow = g.longWindow[1:]
}

dynamicThreshold := g.threshold
if len(g.longWindow) >= 2 {
mean := average(g.longWindow)
stdDev := standardDeviation(g.longWindow, mean)
dynamicThreshold = g.threshold + g.adaptiveFactor*stdDev
if dynamicThreshold > 100 {
dynamicThreshold = 100
}
}

return float64(score) >= dynamicThreshold && g.detector.IsStable()
}

// ExportState capture l'état interne pour la reconstruction.
func (g *StabilityGate) ExportState() GateState {
windowCopy := make([]float64, len(g.longWindow))
copy(windowCopy, g.longWindow)
detectorWindow := make([]float64, len(g.detector.window))
copy(detectorWindow, g.detector.window)
return GateState{
LongWindow:      windowCopy,
DetectorWindow:  detectorWindow,
Threshold:       g.threshold,
VarianceThresh:  g.varianceThreshold,
AdaptiveFactor:  g.adaptiveFactor,
MaxWindow:       g.maxWindow,
}
}

// ImportState restaure l'état interne (pour le replay).
func (g *StabilityGate) ImportState(state GateState) {
g.threshold = state.Threshold
g.varianceThreshold = state.VarianceThresh
g.adaptiveFactor = state.AdaptiveFactor
g.maxWindow = state.MaxWindow
g.longWindow = make([]float64, len(state.LongWindow))
copy(g.longWindow, state.LongWindow)
g.detector = NewAntiCollapseDetector(g.maxWindow, g.varianceThreshold)
g.detector.window = make([]float64, len(state.DetectorWindow))
copy(g.detector.window, state.DetectorWindow)
}

// --- utilitaires mathématiques (purs) ---
func average(data []float64) float64 {
sum := 0.0
for _, v := range data {
sum += v
}
return sum / float64(len(data))
}

func standardDeviation(data []float64, mean float64) float64 {
sumSquares := 0.0
for _, v := range data {
diff := v - mean
sumSquares += diff * diff
}
return math.Sqrt(sumSquares / float64(len(data)))
}
