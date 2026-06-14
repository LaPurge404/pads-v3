package storage

import "database/sql"

func (db *DB) InsertNode(id, nodeType, filePath, signatureHash string) error {
    _, err := db.SQL.Exec(
        "INSERT OR IGNORE INTO nodes (id, type, file_path, signature_hash) VALUES (?, ?, ?, ?)",
        id, nodeType, filePath, signatureHash,
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
