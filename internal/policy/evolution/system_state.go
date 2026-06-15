package evolution

// BanditState capture l'état interne du bandit pour la reconstruction.
type BanditState struct {
Arms map[string]float64
Seed int64
}

// GateState capture l'état complet du StabilityGate.
type GateState struct {
LongWindow     []float64 // fenêtre longue pour le seuil adaptatif
DetectorWindow []float64 // fenêtre du détecteur d'instabilité
Threshold      float64
VarianceThresh float64
AdaptiveFactor float64
MaxWindow      int
}

// SystemState représente l'état intégral du moteur d'évolution à un instant T.
type SystemState struct {
Bandit         BanditState
Gate           GateState
DetectorWindow []float64 // fenêtre de l'AntiCollapseDetector (peut être redondant, conservé pour cohérence)
Mode           Mode
Sequence       int // dernière séquence traitée
}
