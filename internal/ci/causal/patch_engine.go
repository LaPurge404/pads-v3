package causal

import (
	"fmt"

	"pads-v3/internal/ci"
)

// PatchAction represents a corrective action to apply to a job or step.
type PatchAction struct {
	Kind   string // "invalidate_cache", "force_recompute", "reorder_job", "reset_step"
	JobID  string
	StepID string
	Detail string
}

// PatchEngine translates a DivergenceNode into a corrective PatchAction.
type PatchEngine struct {
	Cache *ci.Cache
}

// GeneratePatch analyses a divergence node and returns the recommended action.
func (pe *PatchEngine) GeneratePatch(node *DivergenceNode) (*PatchAction, error) {
	if node == nil {
		return nil, nil
	}

	switch node.Type {
	case DiffCacheHit:
		// Cache inconsistency: invalidate the cache key for this step
		return &PatchAction{
			Kind:   "invalidate_cache",
			JobID:  node.JobID,
			StepID: node.StepID,
			Detail: fmt.Sprintf("cache divergence for %s/%s: %s", node.JobID, node.StepID, node.Detail),
		}, nil

	case DiffArtifact:
		// Artifact mismatch: force recompute the step
		return &PatchAction{
			Kind:   "force_recompute",
			JobID:  node.JobID,
			StepID: node.StepID,
			Detail: fmt.Sprintf("artifact mismatch for %s/%s: %s", node.JobID, node.StepID, node.Detail),
		}, nil

	case DiffOrdering:
		// Ordering mismatch: reset the job's step execution context
		return &PatchAction{
			Kind:   "reset_step",
			JobID:  node.JobID,
			StepID: node.StepID,
			Detail: fmt.Sprintf("ordering mismatch for %s/%s: %s", node.JobID, node.StepID, node.Detail),
		}, nil

	case DiffMissing:
		// Missing event: force recompute the missing step
		return &PatchAction{
			Kind:   "force_recompute",
			JobID:  node.OracleEvent.JobID,
			StepID: node.OracleEvent.StepID,
			Detail: fmt.Sprintf("missing event in replay: %s", node.Detail),
		}, nil

	case DiffExtra:
		// Extra event: reset the step to remove the spurious event
		return &PatchAction{
			Kind:   "reset_step",
			JobID:  node.ReplayEvent.JobID,
			StepID: node.ReplayEvent.StepID,
			Detail: fmt.Sprintf("extra event in replay: %s", node.Detail),
		}, nil

	default:
		return nil, fmt.Errorf("unknown divergence type: %s", node.Type)
	}
}

// ApplyPatch applies the corrective action to the given jobs map.
// This is a minimal implementation; in production, this would interact with the scheduler.
func (pe *PatchEngine) ApplyPatch(jobs *map[string]ci.Job, action *PatchAction) error {
	if action == nil {
		return nil
	}

	switch action.Kind {
	case "invalidate_cache":
		// Invalidate cache for the specific job/step
		// In practice, this would call Cache.Invalidate(key)
		return nil

	case "force_recompute":
		// Force recompute: mark the step as needing re-execution
		if job, ok := (*jobs)[action.JobID]; ok {
			for i, step := range job.Steps {
				if step.ID == action.StepID {
					// Force recompute by clearing the step's cache key
					// This is a placeholder; in practice, we would modify the plan
					_ = i
					return nil
				}
			}
		}
		return fmt.Errorf("step %s not found in job %s", action.StepID, action.JobID)

	case "reset_step":
		// Reset step: clear the step's state and re-execute
		return nil

	default:
		return fmt.Errorf("unknown action kind: %s", action.Kind)
	}
}
