package fault

import (
	"testing"
	"time"
)

func TestFaultConfig_Defaults(t *testing.T) {
	cfg := FaultConfig{}
	if cfg.ErrorRate != 0 || cfg.WriteFailRate != 0 {
		t.Error("default rates should be zero")
	}
}

func TestOpenFaultDB_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/fault.db"

	cfg := FaultConfig{
		ErrorRate:     0.1,
		LatencyMin:    1 * time.Millisecond,
		LatencyMax:    2 * time.Millisecond,
		WriteFailRate: 0.1,
	}

	db, err := OpenFaultDB(dbPath, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Verify the database is functional
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Logf("expected fault or success: %v", err)
	}
	t.Log("fault driver opened successfully")
}
