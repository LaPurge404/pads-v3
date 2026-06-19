package adaptive

import (
	"fmt"
	"log/slog"

	"pads-v3/internal/ci"
	"pads-v3/internal/ci/causal"
	"pads-v3/internal/trace"
)

// MutationContext encapsulates the mutable state of the adaptive loop.
type MutationContext struct {
	Jobs      *map[string]ci.Job
	Scheduler *ci.Scheduler
	Cache     *ci.Cache
}

// AdaptiveLoopV2 is the self-correcting execution loop with causal diagnostics.
type AdaptiveLoopV2 struct {
	Scheduler     *ci.Scheduler
	OracleWALPath string
	MaxRetries    int
	MutationCtx   *MutationContext
}

// Run executes the adaptive loop. It returns nil if convergence is achieved.
func (l *AdaptiveLoopV2) Run() error {
	if l.MaxRetries <= 0 {
		l.MaxRetries = 3
	}

	oracleEvents, err := trace.ReadWALFile(l.OracleWALPath)
	if err != nil {
		return fmt.Errorf("adaptive: load oracle: %w", err)
	}
	oracleCausal := causal.InstrumentEvents(oracleEvents)

	for attempt := 1; attempt <= l.MaxRetries; attempt++ {
		slog.Info("adaptive v2: attempt", "attempt", attempt, "max_retries", l.MaxRetries)

		if l.MutationCtx != nil && l.MutationCtx.Jobs != nil {
			if err := l.Scheduler.Run(*l.MutationCtx.Jobs); err != nil {
				slog.Warn("adaptive v2: execution failed", "err", err)
				continue
			}
		} else {
			return fmt.Errorf("adaptive v2: no jobs to execute")
		}

		walPath := l.Scheduler.GetWALPath()
		if walPath == "" {
			return fmt.Errorf("adaptive v2: scheduler has no WAL path")
		}
		currentEvents, err := trace.ReadWALFile(walPath)
		if err != nil {
			return fmt.Errorf("adaptive v2: read current WAL: %w", err)
		}
		currentCausal := causal.InstrumentEvents(currentEvents)

		node := causal.LocateDivergence(oracleCausal, currentCausal)
		if node == nil {
			slog.Info("adaptive v2: convergence achieved", "attempt", attempt)
			return nil
		}

		slog.Warn("adaptive v2: divergence detected", "index", node.Index, "detail", node.Detail)

		engine := &causal.PatchEngine{}
		action, err := engine.GeneratePatch(node)
		if err != nil {
			slog.Warn("adaptive v2: patch generation failed", "err", err)
			continue
		}
		if action == nil {
			slog.Warn("adaptive v2: no patch action generated")
			continue
		}

		slog.Info("adaptive v2: applying patch", "kind", action.Kind, "job", action.JobID, "step", action.StepID)

		if l.MutationCtx != nil {
			if err := engine.ApplyPatch(l.MutationCtx.Jobs, action); err != nil {
				slog.Warn("adaptive v2: patch application failed", "err", err)
				continue
			}
		}
	}

	return fmt.Errorf("adaptive v2: failed to converge after %d attempts", l.MaxRetries)
}
