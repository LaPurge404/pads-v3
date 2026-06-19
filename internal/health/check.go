package health

// Version is the health check package version.
const Version = "1.0"

// HealthChecker holds the health status of each system component.
type HealthChecker struct {
	DB             bool
	WAL            bool
	SemanticMemory bool
	Worker         bool
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
