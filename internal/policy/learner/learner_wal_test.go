package learner

import (
	"fmt"
	"testing"

	"pads-v3/internal/policy"
	"pads-v3/internal/policy/wal"
)

func TestLearnFromWAL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/policy.log"

	w, err := wal.NewPolicyWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		event := wal.PolicyEvent{
			DecisionID: fmt.Sprintf("trace-%d", i),
			Score:      85.0,
			Status:     "PASS",
			Trace: policy.PolicyTrace{
				FinalScore:  85.0,
				FinalStatus: policy.StatusPass,
			},
		}
		if i%2 == 0 {
			event.Trace.Steps = append(event.Trace.Steps, policy.TraceStep{
				Stage:       "GATES",
				FailedGates: []string{"syntax_gate"},
			})
		}
		w.Append(event)
	}

	l := NewLearner()
	l.MinSamples = 3
	config := TunedConfig{
		GateWeights:   map[string]int{"syntax_gate": 30},
		ThresholdPass: 90,
		ThresholdWarn: 70,
		ThresholdFail: 50,
	}

	tuned, report, err := l.LearnFromWAL(w, config)
	if err != nil {
		t.Fatal(err)
	}
	if tuned == nil {
		t.Fatal("tuned config is nil")
	}
	if report.AnomalyScore < 0 {
		t.Error("anomaly score should be non-negative")
	}
	t.Logf("Learned from WAL: anomaly=%.2f, adjustments=%d", report.AnomalyScore, len(report.Adjustments))
}
