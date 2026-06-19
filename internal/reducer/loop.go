package reducer

import (
	"fmt"

	"pads-v3/internal/storage"
)

// RunReductionLoop rebuilds L3 deterministically from the event log (L2).
func RunReductionLoop(db *storage.DB) (int, error) {
	rows, err := db.Query(`
        SELECT sequence_id, event_id, event_type, payload, exit_code
        FROM events
        ORDER BY sequence_id ASC
    `)
	if err != nil {
		return 0, fmt.Errorf("loop query events: %w", err)
	}

	type eventRow struct {
		seq       int64
		eventID   string
		eventType string
		payload   string
		exitCode  int
	}

	var events []eventRow
	for rows.Next() {
		var e eventRow
		if err := rows.Scan(&e.seq, &e.eventID, &e.eventType, &e.payload, &e.exitCode); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("loop iteration error: %w", err)
	}
	rows.Close()

	state := make(map[string]storage.GraphState)
	processed := 0

	for _, e := range events {
		nodeIDs, err := db.GetEventNodeIDs(e.eventID)
		if err != nil {
			return 0, fmt.Errorf("get node ids for %s: %w", e.eventID, err)
		}

		event := storage.Event{
			SequenceID: e.seq,
			EventID:    e.eventID,
			EventType:  e.eventType,
			Payload:    e.payload,
			ExitCode:   e.exitCode,
			NodeIDs:    nodeIDs,
		}

		patch := Reduce(event, state)
		for nodeID, gs := range patch {
			state[nodeID] = gs
		}
		processed++
	}

	if err := db.ClearGraphState(); err != nil {
		return 0, fmt.Errorf("clear state: %w", err)
	}
	for _, gs := range state {
		if err := db.UpsertGraphState(gs); err != nil {
			return 0, fmt.Errorf("upsert state %s: %w", gs.NodeID, err)
		}
	}

	return processed, nil
}
