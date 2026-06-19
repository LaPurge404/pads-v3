package agent

import (
	"fmt"
	"log/slog"

	"pads-v3/internal/storage"
)

// BuildTasks finds all BROKEN nodes and creates a Task for each.
func BuildTasks(db *storage.DB) ([]Task, error) {
	rows, err := db.Query(`
        SELECT node_id, file_path
        FROM graph_state
        JOIN nodes ON graph_state.node_id = nodes.id
        WHERE graph_state.state = 'BROKEN'
    `)
	if err != nil {
		return nil, fmt.Errorf("build tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var nodeID, filePath string
		if err := rows.Scan(&nodeID, &filePath); err != nil {
			return nil, fmt.Errorf("build tasks: scan: %w", err)
		}
		tasks = append(tasks, Task{
			Kind:   TaskFixBroken,
			Target: filePath,
			Goal:   fmt.Sprintf("Repair node %s in %s", nodeID, filePath),
		})
	}
	return tasks, nil
}

// RunOnce finds all BROKEN nodes, asks the Agent for a Plan, and logs the actions.
func RunOnce(db *storage.DB, a Agent) error {
	tasks, err := BuildTasks(db)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	for _, task := range tasks {
		ctx, err := BuildContext(db, task)
		if err != nil {
			slog.Warn("agent: build context error", "err", err)
			continue
		}

		plan, err := a.Solve(task, ctx)
		if err != nil {
			slog.Warn("agent: solve error", "err", err)
			continue
		}

		for _, action := range plan.Steps {
			slog.Info("agent action", "kind", action.Kind, "target", action.Target)
		}
	}

	return nil
}
