package resolver

import (
	"database/sql"
	"fmt"
	"strings"

	"pads-v3/internal/storage"
)

// ResolveCalls transforms all edges of type CALLS whose target starts with "unresolved:"
// into resolved edges pointing to a concrete node in the same package.
func ResolveCalls(db *storage.DB) (int, error) {
	// 1. Fetch all unresolved CALLS edges
	rows, err := db.Query(`
        SELECT rowid, source, target FROM edges
        WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'
        ORDER BY source, target
    `)
	if err != nil {
		return 0, fmt.Errorf("ResolveCalls: query: %w", err)
	}
	defer rows.Close()

	type callEdge struct {
		rowid  int
		source string
		target string
	}
	var unresolved []callEdge
	for rows.Next() {
		var rowid int
		var s, t string
		if err := rows.Scan(&rowid, &s, &t); err != nil {
			return 0, fmt.Errorf("ResolveCalls: scan: %w", err)
		}
		unresolved = append(unresolved, callEdge{rowid: rowid, source: s, target: t})
	}
	rows.Close()

	resolved := 0

	// 2. Process each unresolved edge
	for _, edge := range unresolved {
		identifier := strings.TrimPrefix(edge.target, "unresolved:")

		// Extract package from source
		sourceParts := strings.SplitN(edge.source, ".", 3)
		if len(sourceParts) < 2 {
			continue
		}
		pkg := sourceParts[0]

		// Build candidate resolved IDs in priority order:
		// 1) pkg.Identifier (function)
		// 2) pkg.*.Identifier (pointer receiver method)
		// 3) pkg.*.Identifier (value receiver method - same pattern)
		candidates := []string{
			fmt.Sprintf("%s.%s", pkg, identifier),
			fmt.Sprintf("%s.%%.%s", pkg, identifier),
		}

		var resolvedID string
		for _, pattern := range candidates {
			err := db.QueryRow(
				`SELECT id FROM nodes WHERE id LIKE ? ORDER BY id LIMIT 1`,
				pattern,
			).Scan(&resolvedID)
			if err == nil {
				break
			}
		}

		if resolvedID == "" {
			continue // no match found, leave unresolved
		}

		// 3. Replace edge inside a transaction
		err = db.WithTransaction(func(tx *sql.Tx) error {
			// Delete old unresolved edge
			if _, err := tx.Exec(
				`DELETE FROM edges WHERE rowid = ?`,
				edge.rowid,
			); err != nil {
				return err
			}
			// Insert new resolved edge (INSERT OR IGNORE for idempotence)
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO edges (source, target, relation) VALUES (?, ?, 'CALLS')`,
				edge.source, resolvedID,
			); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			continue
		}
		resolved++
	}

	return resolved, nil
}
