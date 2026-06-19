package agent

import (
	"fmt"

	"pads-v3/internal/storage"
)

// BuildContext creates a Context for a given task by querying the database.
func BuildContext(db *storage.DB, task Task) (Context, error) {
	ctx := Context{
		FilePath: task.Target,
	}

	if db == nil {
		return ctx, nil
	}

	// Get package path from nodes table
	if task.Target != "" {
		row := db.QueryRow(`
			SELECT package_path FROM nodes WHERE file_path = ? LIMIT 1
		`, task.Target)
		var pkgPath string
		if err := row.Scan(&pkgPath); err == nil {
			ctx.PackagePath = pkgPath
		}
	}

	// Get node ID from graph_state for this file
	if task.Target != "" {
		row := db.QueryRow(`
			SELECT node_id FROM graph_state
			JOIN nodes ON graph_state.node_id = nodes.id
			WHERE nodes.file_path = ?
			LIMIT 1
		`, task.Target)
		var nodeID string
		if err := row.Scan(&nodeID); err == nil {
			ctx.NodeID = nodeID
		}
	}

	// Fetch recent L2 events for this file
	rows, err := db.Query(`
		SELECT e.event_id, e.event_type, e.payload, e.exit_code
		FROM events e
		JOIN event_nodes en ON e.event_id = en.event_id
		JOIN nodes n ON en.node_id = n.id
		WHERE n.file_path = ?
		ORDER BY e.sequence_id DESC
		LIMIT 10
	`, task.Target)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ev storage.Event
			if err := rows.Scan(&ev.EventID, &ev.EventType, &ev.Payload, &ev.ExitCode); err == nil {
				ctx.L2Events = append(ctx.L2Events, ev)
			}
		}
	}

	// Build L3State summary by aggregating graph_state for this file
	row := db.QueryRow(`
		SELECT
			COUNT(*),
			COUNT(CASE WHEN gs.state = 'BROKEN' THEN 1 END),
			COUNT(CASE WHEN gs.state = 'STABLE' THEN 1 END)
		FROM graph_state gs
		JOIN nodes n ON gs.node_id = n.id
		WHERE n.file_path = ?
	`, task.Target)
	var total, broken, stable int
	if err := row.Scan(&total, &broken, &stable); err == nil {
		ctx.L3State = storage.GraphState{
			NodeID: task.Target, // reuse field to carry summary
			State:  fmt.Sprintf("total=%d,broken=%d,stable=%d", total, broken, stable),
		}
	}

	return ctx, nil
}
