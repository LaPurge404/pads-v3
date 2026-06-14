package storage

import "database/sql"

func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
    return db.SQL.Query(query, args...)
}

func (db *DB) QueryRow(query string, args ...any) *sql.Row {
    return db.SQL.QueryRow(query, args...)
}
