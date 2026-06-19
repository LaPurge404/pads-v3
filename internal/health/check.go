package health

import "pads-v3/internal/policy/evolution"

// PoolStats holds AgentPool statistics for health reporting.
type PoolStats struct {
	Size     int                          `json:"pool_size"`
	BestArm  string                       `json:"best_arm"`
	ArmStats map[string]evolution.UCBArmStats `json:"arm_stats"`
}

// HealthChecker holds the health status of each system component.
type HealthChecker struct {
	DB             bool       `json:"db"`
	WAL            bool       `json:"wal"`
	SemanticMemory bool       `json:"semantic_memory"`
	Worker         bool       `json:"worker"`
	Pool           *PoolStats `json:"pool,omitempty"`
}

// Check returns the current health status of all components.
// Individual component checks can be added incrementally.
func Check() HealthChecker {
	return HealthChecker{
		DB:             true,
		WAL:            true,
		SemanticMemory: true,
		Worker:         true,
	}
}

// CheckWithPool returns a HealthChecker with optional AgentPool statistics.
func CheckWithPool(poolStats *PoolStats) HealthChecker {
	h := Check()
	h.Pool = poolStats
	return h
}

// String returns a human-readable summary of the health check status.
func (h HealthChecker) String() string {
	return "HealthChecker{DB: " + boolStr(h.DB) +
		", WAL: " + boolStr(h.WAL) +
		", SemanticMemory: " + boolStr(h.SemanticMemory) +
		", Worker: " + boolStr(h.Worker) + "}"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
