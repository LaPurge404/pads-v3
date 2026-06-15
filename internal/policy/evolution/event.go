package evolution

// Event est l'unité atomique déterministe enregistrée dans le journal.
type Event struct {
Sequence       int
CandidateScore int
CurrentScore   int
Weight         float64
Mode           Mode
BanditSeed     int64 // seed utilisée par le bandit pour ce cycle
// Champs pour le replay déterministe du StabilityGate
GateVariance  float64 // variance calculée par le détecteur
GateThreshold float64 // seuil dynamique utilisé
}
