package ci

import (
	"fmt"

	"pads-v3/internal/storage"
)

// GateResult is a pure validation result.
type GateResult struct {
	OK     bool
	Reason string
}

// Validate is a PURE READ-ONLY CHECK.
// It does NOT execute the engine, it only checks invariants on the current state.
func Validate(db *storage.DB) (GateResult, error) {
	// 1. Hard invariant: no BROKEN nodes
	var brokenCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE state = 'BROKEN'`).Scan(&brokenCount)
	if err != nil {
		return GateResult{OK: false, Reason: "failed to query graph_state"}, err
	}
	if brokenCount > 0 {
		return GateResult{OK: false, Reason: fmt.Sprintf("CI GATE FAILED: %d BROKEN nodes", brokenCount)}, nil
	}

	// 2. Consistency: every node must have a state
	var missing int
	err = db.QueryRow(`
        SELECT COUNT(*)
        FROM nodes n
        LEFT JOIN graph_state g ON n.id = g.node_id
        WHERE g.node_id IS NULL
    `).Scan(&missing)
	if err != nil {
		return GateResult{OK: false, Reason: "failed to validate node coverage"}, err
	}
	if missing > 0 {
		return GateResult{OK: false, Reason: fmt.Sprintf("CI GATE FAILED: %d nodes missing state", missing)}, nil
	}

	return GateResult{OK: true}, nil
}
