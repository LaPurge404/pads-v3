package storage

import "database/sql"

func (db *DB) InsertNode(id, nodeType, filePath, signatureHash, fileHash string) error {
    _, err := db.SQL.Exec(
        "INSERT OR IGNORE INTO nodes (id, type, file_path, signature_hash, file_hash) VALUES (?, ?, ?, ?, ?)",
        id, nodeType, filePath, signatureHash, fileHash,
    )
    return err
}

func (db *DB) InsertEdge(source, target, relation string) error {
    _, err := db.SQL.Exec(
        "INSERT OR IGNORE INTO edges (source, target, relation) VALUES (?, ?, ?)",
        source, target, relation,
    )
    return err
}

func (db *DB) ClearFileNodes(filePath string) error {
    return db.WithTransaction(func(tx *sql.Tx) error {
        if _, err := tx.Exec(
            `DELETE FROM edges
             WHERE source IN (SELECT id FROM nodes WHERE file_path = ?)
                OR target IN (SELECT id FROM nodes WHERE file_path = ?)`,
            filePath, filePath,
        ); err != nil {
            return err
        }
        if _, err := tx.Exec(`DELETE FROM nodes WHERE file_path = ?`, filePath); err != nil {
            return err
        }
        return nil
    })
}

// GetFileHash returns the stored file_hash for a node, or empty string if not found.
func (db *DB) GetFileHash(nodeID string) (string, error) {
    var hash string
    err := db.SQL.QueryRow(`SELECT file_hash FROM nodes WHERE id = ?`, nodeID).Scan(&hash)
    if err != nil {
        return "", err
    }
    return hash, nil
}

// UpdateFileHash updates the file_hash for a given file path (all nodes in that file).
func (db *DB) UpdateFileHash(filePath, newHash string) error {
    _, err := db.SQL.Exec(`UPDATE nodes SET file_hash = ? WHERE file_path = ?`, newHash, filePath)
    return err
}
