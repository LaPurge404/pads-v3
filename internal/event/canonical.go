package event

// CanonicalEvent is the ONLY persisted event format from this point forward.
// CanonicalEvent is the sole event contract for the entire system.
type CanonicalEvent struct {
    NodeID  string `json:"node_id"`
    Type    string `json:"type"`
    JobID   string `json:"job_id"`
    StepID  string `json:"step_id"`
    Status  string `json:"status"`
    Payload string `json:"payload"`
    Time    int64  `json:"time"`
}
