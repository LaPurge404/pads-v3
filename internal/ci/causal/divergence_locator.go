package causal

import (
    "fmt"
)

// DivergenceNode pinpoints the first event where two causal traces differ.
type DivergenceNode struct {
    Index       int
    OracleEvent CausalEvent
    ReplayEvent CausalEvent
    JobID       string
    StepID      string
    Type        DiffType
    Detail      string
}

// LocateDivergence compares two causal event streams and returns the first divergence node.
func LocateDivergence(oracle, replay []CausalEvent) *DivergenceNode {
    minLen := len(oracle)
    if len(replay) < minLen {
        minLen = len(replay)
    }

    for i := 0; i < minLen; i++ {
        o := oracle[i]
        r := replay[i]

        // 1. CACHE-specific semantic divergence (must be first)
        if o.Type == "CI_CACHE_HIT" || o.Type == "CI_CACHE_MISS" {
            if o.Type != r.Type || o.Status != r.Status {
                return &DivergenceNode{
                    Index:       i,
                    OracleEvent: o,
                    ReplayEvent: r,
                    JobID:       o.JobID,
                    StepID:      o.StepID,
                    Type:        DiffCacheHit,
                    Detail:      fmt.Sprintf("cache divergence: %s vs %s", o.Status, r.Status),
                }
            }
        }

        // 2. Type mismatch (non-cache events)
        if o.Type != r.Type {
            return &DivergenceNode{
                Index:       i,
                OracleEvent: o,
                ReplayEvent: r,
                JobID:       o.JobID,
                StepID:      o.StepID,
                Type:        DiffOrdering,
                Detail:      fmt.Sprintf("type mismatch: %s vs %s", o.Type, r.Type),
            }
        }

        // 3. Status mismatch (generic)
        if o.Status != r.Status {
            return &DivergenceNode{
                Index:       i,
                OracleEvent: o,
                ReplayEvent: r,
                JobID:       o.JobID,
                StepID:      o.StepID,
                Type:        DiffOrdering,
                Detail:      fmt.Sprintf("status mismatch: %s vs %s", o.Status, r.Status),
            }
        }

        // 4. Payload mismatch
        if o.Payload != r.Payload {
            return &DivergenceNode{
                Index:       i,
                OracleEvent: o,
                ReplayEvent: r,
                JobID:       o.JobID,
                StepID:      o.StepID,
                Type:        DiffArtifact,
                Detail:      fmt.Sprintf("payload mismatch at step %s", o.StepID),
            }
        }

        // 5. Causal integrity check
        if o.CausalID != r.CausalID || o.ParentID != r.ParentID {
            return &DivergenceNode{
                Index:       i,
                OracleEvent: o,
                ReplayEvent: r,
                JobID:       o.JobID,
                StepID:      o.StepID,
                Type:        DiffOrdering,
                Detail:      "causal chain integrity violation",
            }
        }
    }

    if len(oracle) > len(replay) {
        return &DivergenceNode{
            Index:       len(replay),
            OracleEvent: oracle[len(replay)],
            Type:        DiffMissing,
            Detail:      fmt.Sprintf("missing event: %s", oracle[len(replay)].Type),
        }
    }

    if len(replay) > len(oracle) {
        return &DivergenceNode{
            Index:       len(oracle),
            ReplayEvent: replay[len(oracle)],
            Type:        DiffExtra,
            Detail:      fmt.Sprintf("extra event: %s", replay[len(oracle)].Type),
        }
    }

    return nil
}
