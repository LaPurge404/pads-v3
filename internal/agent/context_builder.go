package agent

import "pads-v3/internal/storage"

// BuildContext creates a Context for a given task by querying the database.
func BuildContext(db *storage.DB, task Task) (Context, error) {
    ctx := Context{
        FilePath: task.Target,
    }

    // Enrich with L3 state for the target file in the future
    // For now, return a minimal context
    return ctx, nil
}
