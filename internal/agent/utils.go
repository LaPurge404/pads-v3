package agent

import (
	"strings"
)

// SerializePlan converts a Plan to a human-readable string representation.
func SerializePlan(plan Plan) string {
	var parts []string
	for i, step := range plan.Steps {
		parts = append(parts, formatStep(i, step))
	}
	return strings.Join(parts, "; ")
}

// ComputeSandboxScore converts sandbox results into an evolution score.
func ComputeSandboxScore(res SandboxResult) int {
	score := 0

	if res.Error == nil && !strings.Contains(res.BuildOutput, "error") {
		score += 30 // Build success
	}

	if res.Passed {
		score += 50 // All tests passing
	} else if res.TestsPassed > 0 {
		total := res.TestsPassed + res.TestsFailed
		if total > 0 {
			score += 50 * res.TestsPassed / total
		}
	}

	if !strings.Contains(res.BuildOutput, "warning") && !strings.Contains(res.TestOutput, "warning") {
		score += 20
	}

	return score
}

// formatStep formats a single plan step for SerializePlan.
// Extracted as a separate function to allow reuse without the loop overhead.
func formatStep(i int, step Action) string {
	return "step" + itoa(i) + ": " + string(step.Kind) + " -> " + step.Target
}

// itoa converts an int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
