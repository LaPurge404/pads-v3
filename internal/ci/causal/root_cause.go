package causal

// RootCause represents the identified cause of a divergence.
type RootCause struct {
	StepID   string
	Reason   string
	Severity int // 1 = minor, 2 = structural, 3 = critical
}

// AnalyzeCause interprets a CausalDiff and returns a human-readable root cause.
func AnalyzeCause(diff CausalDiff) RootCause {
	switch diff.Type {
	case DiffCacheHit:
		return RootCause{
			StepID:   diff.Oracle.StepID,
			Reason:   "cache hit/miss mismatch: deterministic input drift or cache state divergence",
			Severity: 2,
		}
	case DiffOrdering:
		return RootCause{
			StepID:   diff.Oracle.StepID,
			Reason:   "event ordering mismatch: DAG scheduling instability or non-deterministic execution order",
			Severity: 3,
		}
	case DiffArtifact:
		return RootCause{
			StepID:   diff.Oracle.StepID,
			Reason:   "artifact payload mismatch: filesystem divergence or non-canonical artifact path",
			Severity: 1,
		}
	case DiffMissing:
		return RootCause{
			StepID:   diff.Oracle.StepID,
			Reason:   "missing event in replay: execution path truncated or cache hit unexpectedly",
			Severity: 2,
		}
	case DiffExtra:
		return RootCause{
			StepID:   diff.Replay.StepID,
			Reason:   "extra event in replay: non-deterministic execution branch or cache miss incorrectly triggered",
			Severity: 2,
		}
	default:
		return RootCause{
			Reason: "no divergence detected",
		}
	}
}
