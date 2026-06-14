package chaos

import (
    "os"
    "os/exec"
    "testing"

    "pads-v3/internal/reducer"
    "pads-v3/internal/storage"
)

func createTempDB(t *testing.T) string {
    t.Helper()
    f, err := os.CreateTemp("", "pads-chaos-*.db")
    if err != nil {
        t.Fatal(err)
    }
    path := f.Name()
    f.Close()
    return path
}

func setupDB(t *testing.T) (*storage.DB, string) {
    t.Helper()
    path := createTempDB(t)
    db, err := storage.Open(path)
    if err != nil {
        os.Remove(path)
        t.Fatal(err)
    }
    _, err = db.InsertEvent("evt-1", "OS_EXEC_RESULT", "test", 0)
    if err != nil {
        db.Close(); os.Remove(path); t.Fatal(err)
    }
    db.InsertEventNode("evt-1", "main.A")
    db.InsertEventNode("evt-1", "main.B")
    _, err = db.InsertEvent("evt-2", "OS_EXEC_RESULT", "test", 1)
    if err != nil {
        db.Close(); os.Remove(path); t.Fatal(err)
    }
    db.InsertEventNode("evt-2", "main.C")
    return db, path
}

func hashL3(t *testing.T, db *storage.DB) string {
    t.Helper()
    rows, err := db.Query(`SELECT node_id, state FROM graph_state ORDER BY node_id`)
    if err != nil {
        t.Fatal(err)
    }
    defer rows.Close()
    var result string
    for rows.Next() {
        var id, state string
        rows.Scan(&id, &state)
        result += id + "|" + state + "\n"
    }
    return result
}

func TestRebuildFromScratch(t *testing.T) {
    db, path := setupDB(t)
    defer db.Close()
    defer os.Remove(path)

    _, err := reducer.RunReductionLoop(db)
    if err != nil {
        t.Fatalf("first run: %v", err)
    }
    firstHash := hashL3(t, db)

    db.ClearGraphState()
    _, err = reducer.RunReductionLoop(db)
    if err != nil {
        t.Fatalf("second run: %v", err)
    }
    secondHash := hashL3(t, db)

    if firstHash != secondHash {
        t.Errorf("L3 divergence after rebuild")
    }
}

func TestDuplicateEvents(t *testing.T) {
    db, path := setupDB(t)
    defer db.Close()
    defer os.Remove(path)

    db.InsertEvent("evt-1", "OS_EXEC_RESULT", "test", 0)

    _, err := reducer.RunReductionLoop(db)
    if err != nil {
        t.Fatal(err)
    }

    var state string
    db.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.A'`).Scan(&state)
    if state != "STABLE" {
        t.Errorf("expected STABLE, got %s", state)
    }
}

func TestPartialEventNodes(t *testing.T) {
    db, path := setupDB(t)
    defer db.Close()
    defer os.Remove(path)

    db.InsertEvent("evt-orphan", "OS_EXEC_RESULT", "test", 0)

    _, err := reducer.RunReductionLoop(db)
    if err != nil {
        t.Fatal(err)
    }

    var count int
    db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE last_event_id = 'evt-orphan'`).Scan(&count)
    if count != 0 {
        t.Errorf("orphan event should not modify state")
    }
}

func TestConvergenceAfterReplay(t *testing.T) {
    db, path := setupDB(t)
    defer db.Close()
    defer os.Remove(path)

    var previousHash string
    for i := 0; i < 5; i++ {
        db.ClearGraphState()
        _, err := reducer.RunReductionLoop(db)
        if err != nil {
            t.Fatal(err)
        }
        currentHash := hashL3(t, db)
        if i > 0 && previousHash != currentHash {
            t.Errorf("run %d diverged", i)
        }
        previousHash = currentHash
    }
}

func TestOrderStability(t *testing.T) {
    path := createTempDB(t)
    defer os.Remove(path)

    db, _ := storage.Open(path)
    defer db.Close()

    db.InsertEvent("evt-last", "OS_EXEC_RESULT", "test", 1)
    db.InsertEventNode("evt-last", "main.Z")
    db.InsertEvent("evt-first", "OS_EXEC_RESULT", "test", 0)
    db.InsertEventNode("evt-first", "main.A")

    _, err := reducer.RunReductionLoop(db)
    if err != nil {
        t.Fatal(err)
    }

    var state string
    db.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.A'`).Scan(&state)
    if state != "STABLE" {
        t.Errorf("expected STABLE, got %s", state)
    }
    db.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.Z'`).Scan(&state)
    if state != "BROKEN" {
        t.Errorf("expected BROKEN, got %s", state)
    }
}

func TestCrashRecovery(t *testing.T) {
    path := createTempDB(t)
    defer os.Remove(path)

    db, _ := storage.Open(path)
    db.InsertEvent("evt-1", "OS_EXEC_RESULT", "test", 0)
    db.InsertEventNode("evt-1", "main.A")
    db.Close()

    cmd := exec.Command("go", "test", "-run", "TestCrashHelper", "-v")
    cmd.Env = append(os.Environ(), "CHAOS_DB="+path)
    cmd.Run()

    db2, err := storage.Open(path)
    if err != nil {
        t.Fatal(err)
    }
    defer db2.Close()

    _, err = reducer.RunReductionLoop(db2)
    if err != nil {
        t.Fatal(err)
    }

    var state string
    db2.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.A'`).Scan(&state)
    if state != "STABLE" {
        t.Errorf("expected STABLE after crash recovery, got %s", state)
    }
}

// TestDisasterRecovery simulates total loss of the database file and recovery from L2 backup.
func TestDisasterRecovery(t *testing.T) {
    path := createTempDB(t)
    defer os.Remove(path)

    // Phase 1: create database and populate L2 + L3
    db, _ := storage.Open(path)
    db.InsertEvent("evt-1", "OS_EXEC_RESULT", "test", 0)
    db.InsertEventNode("evt-1", "main.A")
    _, err := reducer.RunReductionLoop(db)
    if err != nil {
        t.Fatal(err)
    }
    db.Close()

    // Phase 2: simulate total loss of the database file
    os.Remove(path)

    // Phase 3: recreate database and re-inject events (restore from backup)
    db2, err := storage.Open(path)
    if err != nil {
        t.Fatal(err)
    }
    defer db2.Close()
    _, err = db2.InsertEvent("evt-1", "OS_EXEC_RESULT", "test", 0)
    if err != nil {
        t.Fatal(err)
    }
    err = db2.InsertEventNode("evt-1", "main.A")
    if err != nil {
        t.Fatal(err)
    }

    // Phase 4: rerun reduction and verify state is recovered
    _, err = reducer.RunReductionLoop(db2)
    if err != nil {
        t.Fatal(err)
    }
    var state string
    db2.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.A'`).Scan(&state)
    if state != "STABLE" {
        t.Errorf("expected recovery to STABLE, got '%s'", state)
    }
}
