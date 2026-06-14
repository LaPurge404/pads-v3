package ci

import (
    "testing"

    "pads-v3/internal/storage"
)

func setupTestDB(t *testing.T) *storage.DB {
    db, err := storage.Open(":memory:")
    if err != nil {
        t.Fatal(err)
    }
    // Ensure tables exist (storage.Open should create them, but we make sure)
    db.SQL.Exec(`CREATE TABLE IF NOT EXISTS graph_state (node_id TEXT PRIMARY KEY, state TEXT)`)
    db.SQL.Exec(`CREATE TABLE IF NOT EXISTS nodes (id TEXT PRIMARY KEY, type TEXT, file_path TEXT, signature_hash TEXT, file_hash TEXT)`)
    return db
}

func TestGateRejectsBrokenState(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    // Inject a BROKEN node directly
    _, err := db.SQL.Exec(`INSERT INTO graph_state(node_id, state) VALUES ('n1', 'BROKEN')`)
    if err != nil {
        t.Fatal(err)
    }

    res, err := Validate(db)
    if err != nil {
        t.Fatal(err)
    }
    if res.OK {
        t.Error("expected gate to reject BROKEN state")
    }
}

func TestGateAcceptsCleanState(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    // Insert a node and its STABLE state
    _, err := db.SQL.Exec(`INSERT INTO nodes(id, type, file_path, signature_hash, file_hash) VALUES ('n1', 'func', 'test.go', 'abc', 'def')`)
    if err != nil {
        t.Fatal(err)
    }
    _, err = db.SQL.Exec(`INSERT INTO graph_state(node_id, state) VALUES ('n1', 'STABLE')`)
    if err != nil {
        t.Fatal(err)
    }

    res, err := Validate(db)
    if err != nil {
        t.Fatal(err)
    }
    if !res.OK {
        t.Errorf("expected gate to accept clean state, got: %s", res.Reason)
    }
}
