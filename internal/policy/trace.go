package policy

import "fmt"

// PolicyTrace is a machine-readable audit log of the decision process.
type PolicyTrace struct {
	DecisionID  string
	Steps       []TraceStep
	FinalScore  float64
	FinalStatus Status
}

// TraceStep represents a single transformation of the score.
type TraceStep struct {
	Stage       string  // "GATES", "CERTIFICATION", "HARD_FAIL_CAP", "CHAOS", "FINAL"
	Source      string  // gate name / cert / chaos mode
	Impact      float64 // score change
	ScoreBefore float64
	ScoreAfter  float64
	Reason      string
	FailedGates []string // filled when Stage == "GATES"
}

// PolicyExplanation is a human and ML readable summary of the decision.
type PolicyExplanation struct {
	Summary     string
	KeyReasons  []string
	RiskSignals []string
	Confidence  float64
}

var traceCounter int

func generateTraceID() string {
	traceCounter++
	return fmt.Sprintf("trace-%d", traceCounter)
}
