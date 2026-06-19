package storage

type Event struct {
	SequenceID int64
	EventID    string
	EventType  string
	Payload    string
	ExitCode   int
	NodeIDs    []string
}

func (db *DB) InsertEvent(eventID, eventType, payload string, exitCode int) (int64, error) {
	result, err := db.SQL.Exec(
		"INSERT OR IGNORE INTO events (event_id, event_type, payload, exit_code) VALUES (?, ?, ?, ?)",
		eventID, eventType, payload, exitCode,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
