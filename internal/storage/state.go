package storage

const (
	STATE_STABLE   = "STABLE"
	STATE_BROKEN   = "BROKEN"
	STATE_UNTESTED = "UNTESTED"
)

type GraphState struct {
	NodeID         string
	State          string
	LastEventID    string
	LastExitCode   int
	LastStderrHash string
}

func (db *DB) ClearGraphState() error {
	_, err := db.SQL.Exec(`DELETE FROM graph_state`)
	return err
}

func (db *DB) ClearGraphStateByFile(filePath string) error {
	_, err := db.SQL.Exec(`
        DELETE FROM graph_state
        WHERE node_id IN (
            SELECT id FROM nodes WHERE file_path = ?
        )
    `, filePath)
	return err
}

func (db *DB) UpsertGraphState(gs GraphState) error {
	_, err := db.SQL.Exec(
		`INSERT OR REPLACE INTO graph_state (node_id, state, last_event_id, last_exit_code, last_stderr_hash)
         VALUES (?, ?, ?, ?, ?)`,
		gs.NodeID, gs.State, gs.LastEventID, gs.LastExitCode, gs.LastStderrHash,
	)
	return err
}
