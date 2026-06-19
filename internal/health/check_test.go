package health

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCheckFileExists(t *testing.T) {
	t.Run("returns true for existing file", func(t *testing.T) {
		f, err := os.CreateTemp("", "healthtest_*.txt")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		f.Close()

		if got := checkFileExists(f.Name()); !got {
			t.Errorf("checkFileExists(%q) = false, want true", f.Name())
		}
	})

	t.Run("returns true for existing directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "healthtestdir")
		if err != nil {
			t.Fatalf("MkdirTemp: %v", err)
		}
		defer os.RemoveAll(dir)

		if got := checkFileExists(dir); !got {
			t.Errorf("checkFileExists(%q) = false, want true", dir)
		}
	})

	t.Run("returns false for non-existent path", func(t *testing.T) {
		if got := checkFileExists("/nonexistent/health/test/path"); got {
			t.Errorf("checkFileExists(%q) = true, want false", "/nonexistent/health/test/path")
		}
	})

	t.Run("returns false for empty path", func(t *testing.T) {
		if got := checkFileExists(""); got {
			t.Errorf("checkFileExists(%q) = true, want false", "")
		}
	})
}

func TestCheckSQLite(t *testing.T) {
	t.Run("returns true for accessible SQLite file", func(t *testing.T) {
		f, err := os.CreateTemp("", "healthtest_*.db")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		dbPath := f.Name()
		f.Close()
		defer os.Remove(dbPath)

		// Touch the file so it exists, then try to open it with sqlite driver
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Skipf("sqlite3 driver unavailable: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			t.Skipf("cannot ping sqlite3 at %s: %v", dbPath, err)
		}

		if got := checkSQLite(dbPath); !got {
			t.Errorf("checkSQLite(%q) = false, want true", dbPath)
		}
	})

	t.Run("returns false for non-existent path", func(t *testing.T) {
		if got := checkSQLite("/nonexistent/health/test.db"); got {
			t.Errorf("checkSQLite(%q) = true, want false", "/nonexistent/health/test.db")
		}
	})

	t.Run("returns false for empty path", func(t *testing.T) {
		if got := checkSQLite(""); got {
			t.Errorf("checkSQLite(%q) = true, want false", "")
		}
	})
}

func TestCheckerCheck(t *testing.T) {
	t.Run("returns true fields when paths are accessible and workerFn returns true", func(t *testing.T) {
		walFile, err := os.CreateTemp("", "healthtest_wal_*.db")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		walPath := walFile.Name()
		walFile.Close()
		defer os.Remove(walPath)

		semFile, err := os.CreateTemp("", "healthtest_semdb_*.db")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		semPath := semFile.Name()
		semFile.Close()
		defer os.Remove(semPath)

		// Open semDB to make it a valid sqlite file
		db, err := sql.Open("sqlite3", semPath)
		if err == nil {
			db.Close()
		}

		workerCalled := false
		checker := NewChecker(
			Paths{WALPath: walPath, SemDB: semPath},
			func() bool { workerCalled = true; return true },
		)

		result := checker.Check()

		if !result.DB {
			t.Error("Check().DB = false, want true (WAL file exists)")
		}
		if !result.WAL {
			t.Error("Check().WAL = false, want true (WAL file exists)")
		}
		if !result.SemanticMemory {
			t.Error("Check().SemanticMemory = false, want true (SemDB accessible)")
		}
		if !result.Worker {
			t.Error("Check().Worker = false, want true (workerFn returned true)")
		}
		if !workerCalled {
			t.Error("workerFn was not called")
		}
	})

	t.Run("DB and WAL false when WAL path does not exist", func(t *testing.T) {
		checker := NewChecker(
			Paths{WALPath: "/nonexistent/wal", SemDB: "/nonexistent/semdb"},
			func() bool { return true },
		)

		result := checker.Check()

		if result.DB {
			t.Error("Check().DB = true, want false (WAL path does not exist)")
		}
		if result.WAL {
			t.Error("Check().WAL = true, want false (WAL path does not exist)")
		}
	})

	t.Run("Worker true when workerFn is nil", func(t *testing.T) {
		f, err := os.CreateTemp("", "healthtest_nilworker_*.db")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		f.Close()
		defer os.Remove(f.Name())

		checker := NewChecker(
			Paths{WALPath: f.Name(), SemDB: f.Name()},
			nil, // no workerFn
		)

		result := checker.Check()

		if !result.Worker {
			t.Error("Check().Worker = false, want true (workerFn is nil → returns true)")
		}
	})
}

func TestCheckWithPool(t *testing.T) {
	t.Run("enriches HealthChecker with PoolStats", func(t *testing.T) {
		h := HealthChecker{DB: true, WAL: true, SemanticMemory: true, Worker: true}
		poolStats := &PoolStats{Size: 4, BestArm: "ucb1"}

		result := CheckWithPool(h, poolStats)

		if result.Pool == nil {
			t.Fatal("CheckWithPool().Pool is nil, want non-nil PoolStats")
		}
		if result.Pool.Size != 4 {
			t.Errorf("Pool.Size = %d, want 4", result.Pool.Size)
		}
		if result.Pool.BestArm != "ucb1" {
			t.Errorf("Pool.BestArm = %q, want %q", result.Pool.BestArm, "ucb1")
		}
	})

	t.Run("original HealthChecker fields are preserved", func(t *testing.T) {
		h := HealthChecker{DB: true, WAL: false, SemanticMemory: true, Worker: false}
		poolStats := &PoolStats{Size: 1}

		result := CheckWithPool(h, poolStats)

		if !result.DB {
			t.Error("DB field lost after CheckWithPool")
		}
		if result.WAL {
			t.Error("WAL field changed unexpectedly")
		}
	})
}

func TestCheckWithAutonomous(t *testing.T) {
	t.Run("enriches HealthChecker with AutonomousStatus", func(t *testing.T) {
		h := HealthChecker{DB: true, WAL: true, SemanticMemory: true, Worker: true}
		autoStats := &AutonomousStatus{Enabled: true, Cycles: 42}

		result := CheckWithAutonomous(h, autoStats)

		if result.Autonomous == nil {
			t.Fatal("CheckWithAutonomous().Autonomous is nil, want non-nil")
		}
		if !result.Autonomous.Enabled {
			t.Error("Autonomous.Enabled = false, want true")
		}
		if result.Autonomous.Cycles != 42 {
			t.Errorf("Autonomous.Cycles = %d, want 42", result.Autonomous.Cycles)
		}
	})
}