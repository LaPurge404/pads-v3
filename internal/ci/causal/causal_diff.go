package causal

import (
    "fmt"
)

// DiffType categorizes the root cause of a divergence.
type DiffType string

const (
    DiffNone       DiffType = "NONE"
    DiffCacheHit   DiffType = "CACHE_HIT_MISMATCH"
    DiffOrdering   DiffType = "ORDERING_MISMATCH"
    DiffArtifact   DiffType = "ARTIFACT_MISMATCH"
    DiffMissing    DiffType = "MISSING_EVENT"
    DiffExtra      DiffType = "EXTRA_EVENT"
)

// CausalDiff pinpoints the exact node where divergence begins.
type CausalDiff struct {
    Type      DiffType
    Index     int
    Oracle    CausalEvent
    Replay    CausalEvent
    Detail    string
}

// FindFirstDivergentNode compares two causal event streams and returns the first diff.
func FindFirstDivergentNode(oracle, replay []CausalEvent) (CausalDiff, error) {
    minLen := len(oracle)
    if len(replay) < minLen {
        minLen = len(replay)
    }

    for i := 0; i < minLen; i++ {
        o := oracle[i]
        r := replay[i]

        // Type mismatch
        if o.Type != r.Type {
            return CausalDiff{
                Type:   DiffOrdering,
                Index:  i,
                Oracle: o,
                Replay: r,
                Detail: fmt.Sprintf("type mismatch: %s vs %s", o.Type, r.Type),
            }, nil
        }

        // Status mismatch
        if o.Status != r.Status {
            if o.Type == "CI_CACHE_HIT" || o.Type == "CI_CACHE_MISS" {
                return CausalDiff{
                    Type:   DiffCacheHit,
                    Index:  i,
                    Oracle: o,
                    Replay: r,
                    Detail: fmt.Sprintf("cache status mismatch: %s vs %s", o.Status, r.Status),
                }, nil
            }
            return CausalDiff{
                Type:   DiffOrdering,
                Index:  i,
                Oracle: o,
                Replay: r,
                Detail: fmt.Sprintf("status mismatch: %s vs %s", o.Status, r.Status),
            }, nil
        }

        // Payload mismatch (artifact)
        if o.Payload != r.Payload && o.Type == "CI_ARTIFACT" {
            return CausalDiff{
                Type:   DiffArtifact,
                Index:  i,
                Oracle: o,
                Replay: r,
                Detail: fmt.Sprintf("artifact payload mismatch at %s", o.StepID),
            }, nil
        }
    }

    if len(oracle) > len(replay) {
        return CausalDiff{
            Type:   DiffMissing,
            Index:  len(replay),
            Oracle: oracle[len(replay)],
            Detail: fmt.Sprintf("missing event in replay: %s", oracle[len(replay)].Type),
        }, nil
    }
    if len(replay) > len(oracle) {
        return CausalDiff{
            Type:   DiffExtra,
            Index:  len(oracle),
            Replay: replay[len(oracle)],
            Detail: fmt.Sprintf("extra event in replay: %s", replay[len(oracle)].Type),
        }, nil
    }

    return CausalDiff{Type: DiffNone}, nil
}
