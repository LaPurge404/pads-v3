package policy

import (
	"fmt"
	"sort"
)

// ExplainEngine builds audit + explanation layers from policy execution.
type ExplainEngine struct{}

// BuildTrace reconstructs a deterministic execution trace from inputs
// using the same logic as Engine.Evaluate.
func (e *ExplainEngine) BuildTrace(
	gates []GateResult,
	cert *CertificationResult,
	chaos *ChaosReport,
	decision PolicyDecision,
) PolicyTrace {

	trace := PolicyTrace{
		DecisionID:  generateTraceID(),
		Steps:       []TraceStep{},
		FinalScore:  decision.Score,
		FinalStatus: decision.Status,
	}

	// Weights and defaults (must match Engine)
	gateWeights := map[string]int{
		"syntax_gate":      30,
		"semantic_gate":    20,
		"execution_gate":   25,
		"determinism_gate": 25,
	}
	hardFailGates := map[string]bool{
		"execution_gate":   true,
		"determinism_gate": true,
	}
	chaosPenalties := map[string]float64{
		"Silent": 5,
		"Hard":   15,
		"Full":   30,
	}

	// --- Step 1: Evaluate gates ---
	totalWeight := 0
	earnedWeight := 0
	hardFailTriggered := false
	var failedGates []string

	sortedGates := make([]GateResult, len(gates))
	copy(sortedGates, gates)
	sort.Slice(sortedGates, func(i, j int) bool {
		return sortedGates[i].Name < sortedGates[j].Name
	})

	for _, g := range sortedGates {
		w := gateWeights[g.Name]
		if w == 0 {
			w = 10
		}
		totalWeight += w
		if g.Passed {
			earnedWeight += w
		} else {
			failedGates = append(failedGates, g.Name)
			if hardFailGates[g.Name] {
				hardFailTriggered = true
			}
		}
	}

	beforeGates := 100.0
	score := 0.0
	if totalWeight > 0 {
		score = (float64(earnedWeight) / float64(totalWeight)) * 100.0
	}

	trace.Steps = append(trace.Steps, TraceStep{
		Stage:       "GATES",
		Source:      "all_gates",
		Impact:      score - beforeGates,
		ScoreBefore: beforeGates,
		ScoreAfter:  score,
		Reason:      fmt.Sprintf("gates evaluated (failed: %v)", failedGates),
		FailedGates: failedGates,
	})

	// --- Step 2: Certification override ---
	if cert != nil && !cert.Deterministic {
		before := score
		score = 0
		trace.Steps = append(trace.Steps, TraceStep{
			Stage:       "CERTIFICATION",
			Source:      "determinism",
			Impact:      score - before,
			ScoreBefore: before,
			ScoreAfter:  score,
			Reason:      "non-deterministic execution override",
		})
		return trace
	}

	// --- Step 3: Hard-fail gate cap ---
	if hardFailTriggered && score > 60 {
		before := score
		score = 60
		trace.Steps = append(trace.Steps, TraceStep{
			Stage:       "HARD_FAIL_CAP",
			Source:      "hard_fail_gates",
			Impact:      score - before,
			ScoreBefore: before,
			ScoreAfter:  score,
			Reason:      "score capped due to hard-fail gate",
		})
	}

	// --- Step 4: Chaos penalty ---
	if chaos != nil && chaos.Active {
		penalty, ok := chaosPenalties[chaos.Mode]
		if !ok {
			penalty = 0
		}
		before := score
		score -= penalty
		if score < 0 {
			score = 0
		}
		trace.Steps = append(trace.Steps, TraceStep{
			Stage:       "CHAOS",
			Source:      chaos.Mode,
			Impact:      score - before,
			ScoreBefore: before,
			ScoreAfter:  score,
			Reason:      fmt.Sprintf("chaos penalty applied: mode=%s", chaos.Mode),
		})
	}

	// --- Step 5: Final alignment ---
	trace.Steps = append(trace.Steps, TraceStep{
		Stage:       "FINAL",
		Source:      "policy_engine",
		Impact:      0,
		ScoreBefore: score,
		ScoreAfter:  decision.Score,
		Reason:      "final decision alignment",
	})

	return trace
}

// BuildExplanation generates a human-readable explanation from a trace.
func BuildExplanation(trace PolicyTrace) PolicyExplanation {
	var reasons []string
	var risks []string

	for _, step := range trace.Steps {
		if step.Stage == "GATES" && step.Impact < 0 {
			reasons = append(reasons, step.Reason)
		}
		if step.Stage == "CERTIFICATION" {
			risks = append(risks, "determinism violation detected")
		}
		if step.Stage == "CHAOS" {
			risks = append(risks, "system instability detected: "+step.Source)
		}
	}

	confidence := computeConfidence(trace.FinalScore, risks)

	return PolicyExplanation{
		Summary:     summarize(trace),
		KeyReasons:  reasons,
		RiskSignals: risks,
		Confidence:  confidence,
	}
}

// ReplayEngine verifies that a trace is consistent with its final score,
// checking each transition's integrity.
type ReplayEngine struct{}

// Replay replays the trace and returns true if the final score matches
// AND all intermediate deltas are consistent.
func (r *ReplayEngine) Replay(trace PolicyTrace) bool {
	if len(trace.Steps) == 0 {
		return false
	}
	score := trace.Steps[0].ScoreBefore
	for _, step := range trace.Steps {
		if step.Stage == "FINAL" {
			break
		}
		expectedAfter := score + step.Impact
		if step.ScoreAfter != expectedAfter {
			return false
		}
		score = step.ScoreAfter
	}
	return score == trace.FinalScore
}

// --- helpers ---

func computeConfidence(score float64, risks []string) float64 {
	base := score / 100.0
	if len(risks) > 0 {
		base -= 0.1 * float64(len(risks))
	}
	if base < 0 {
		base = 0
	}
	return base
}

func summarize(trace PolicyTrace) string {
	if trace.FinalStatus == StatusPass {
		return "CI run passed all checks"
	}
	if trace.FinalStatus == StatusBlock {
		return "CI run blocked due to critical failures"
	}
	return fmt.Sprintf("CI run completed with status: %s", trace.FinalStatus)
}
