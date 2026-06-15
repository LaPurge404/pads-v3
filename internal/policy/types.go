package policy

// Status represents the final decision of the policy engine.
type Status string

const (
    StatusPass  Status = "PASS"
    StatusWarn  Status = "WARN"
    StatusFail  Status = "FAIL"
    StatusBlock Status = "BLOCK"
)

// Action represents an action decided by the policy engine.
type Action string

const (
    ActionAllow    Action = "ALLOW"
    ActionWarn     Action = "WARN"
    ActionRetry    Action = "RETRY"
    ActionBlock    Action = "BLOCK"
    ActionEscalate Action = "ESCALATE"
)

// PolicyDecision is the output of the policy engine.
type PolicyDecision struct {
    Status    Status   `json:"status"`
    Score     float64  `json:"score"`
    Actions   []Action `json:"actions"`
    Reasoning []string `json:"reasoning"`
}

// GateResult is the input from the gates layer.
type GateResult struct {
    Reason string
    Name     string // e.g., "syntax_gate"
    Passed   bool
    Severity string // "critical", "high", "medium", "low" (optional, for future use)
}

// CertificationResult is the input from the certification layer.
type CertificationResult struct {
    Deterministic bool
    WALHash       string
    ReplayHash    string
}

// ChaosReport is the input from the chaos layer.
type ChaosReport struct {
    Active bool
    Mode   string // "Silent", "Hard", "Full"
    Events []string
}

// TunedConfig holds the parameters that the policy engine uses for scoring.
// It is the data transfer object between the Learner and the ConfigStore.
type TunedConfig struct {
    GateWeights    map[string]int
    ChaosPenalties map[string]float64
    ThresholdPass  float64
    ThresholdWarn  float64
    ThresholdFail  float64
    HardFailGates  map[string]bool
}

// GateInput carries the data needed to evaluate gates and policy.
type GateInput struct {
    Gates []GateResult
    Cert  *CertificationResult
    Chaos *ChaosReport
}
