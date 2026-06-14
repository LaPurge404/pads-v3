package storage

func (db *DB) GetEventNodeIDs(eventID string) ([]string, error) {
    rows, err := db.SQL.Query(
        `SELECT node_id FROM event_nodes WHERE event_id = ? ORDER BY node_id`,
        eventID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []string
    for rows.Next() {
        var id string
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

func (db *DB) InsertEventNode(eventID, nodeID string) error {
    _, err := db.SQL.Exec(
        `INSERT OR IGNORE INTO event_nodes (event_id, node_id) VALUES (?, ?)`,
        eventID, nodeID,
    )
    return err
}
