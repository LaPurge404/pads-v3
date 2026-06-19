package causal

import (
	"fmt"
)

// CausalEvent extends the standard event with causal graph metadata.
type CausalEvent struct {
	Seq     int    `json:"seq"`
	Type    string `json:"type"`
	JobID   string `json:"job_id"`
	StepID  string `json:"step_id"`
	Status  string `json:"status"`
	Payload string `json:"payload"`

	CausalID string `json:"causal_id"`
	ParentID string `json:"parent_id"`
	PhaseID  string `json:"phase_id"`
}

// ComputeCausalID generates a deterministic identifier for causal tracking.
func ComputeCausalID(jobID, stepID, phase string, seq int) string {
	return fmt.Sprintf("%s|%s|%s|%d", jobID, stepID, phase, seq)
}
