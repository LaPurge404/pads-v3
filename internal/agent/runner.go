package agent

import (
    "fmt"
    "log"

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
            Kind:   "fix_broken",
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
        ctx := Context{
            FilePath: task.Target,
        }

        plan, err := a.Solve(task, ctx)
        if err != nil {
            log.Printf("agent: solve error: %v", err)
            continue
        }

        for _, action := range plan.Steps {
            log.Printf("agent action: kind=%s target=%s", action.Kind, action.Target)
        }
    }

    return nil
}
