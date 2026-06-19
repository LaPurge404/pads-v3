package storage

import (
	"os"
	"testing"
)

func TestOpenMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&count); err != nil {
		t.Fatal(err)
	}
	t.Logf("nodes table ready, count=%d", count)
}

func TestInsertAndClearNodes(t *testing.T) {
	tmp, err := os.CreateTemp("", "pads-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Insert a node
	if err := db.InsertNode("main.X", "func", "test.go", "abc123", "def456"); err != nil {
		t.Fatal(err)
	}
	// Insert an edge
	if err := db.InsertEdge("main.X", "main.Y", "CALLS"); err != nil {
		t.Fatal(err)
	}

	// Verify
	var count int
	db.QueryRow("SELECT COUNT(*) FROM nodes WHERE file_path='test.go'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 node, got %d", count)
	}

	// Clear
	if err := db.ClearFileNodes("test.go"); err != nil {
		t.Fatal(err)
	}
	db.QueryRow("SELECT COUNT(*) FROM nodes").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 nodes after clear, got %d", count)
	}
}

func TestGraphState(t *testing.T) {
	db, _ := Open(":memory:")
	defer db.Close()

	// Upsert
	if err := db.UpsertGraphState(GraphState{NodeID: "main.X", State: STATE_STABLE, LastEventID: "ev1", LastExitCode: 0}); err != nil {
		t.Fatal(err)
	}

	var state string
	db.QueryRow("SELECT state FROM graph_state WHERE node_id='main.X'").Scan(&state)
	if state != STATE_STABLE {
		t.Errorf("expected STABLE, got %s", state)
	}

	// Clear
	db.ClearGraphState()
	var count int
	db.QueryRow("SELECT COUNT(*) FROM graph_state").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}
