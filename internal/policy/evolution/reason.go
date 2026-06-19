package evolution

import "fmt"

// TrendDescription returns a human-readable description of a stability trend.
func TrendDescription(slope float64) string {
	switch {
	case slope <= -2.0:
		return "en forte baisse 📉"
	case slope <= -0.5:
		return "en légère baisse ↘️"
	case slope < 0.5:
		return "stable ➡️"
	case slope < 2.0:
		return "en légère hausse ↗️"
	default:
		return "en forte hausse 📈"
	}
}

// BuildReason creates a human-readable reason for a cycle result.
func BuildReason(accepted bool, candidateScore, currentScore int, stabilityDelta float64) string {
	if accepted {
		return fmt.Sprintf("✅ Accepté: candidate=%d, current=%d, Δstabilité=%.2f",
			candidateScore, currentScore, stabilityDelta)
	}
	return fmt.Sprintf("❌ Rejeté: candidate=%d, current=%d, Δstabilité=%.2f",
		candidateScore, currentScore, stabilityDelta)
}
