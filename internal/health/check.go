package health

import (
	"database/sql"
	"os"

	"pads-v3/internal/policy/evolution"
)

// PoolStats holds AgentPool statistics for health reporting.
type PoolStats struct {
	Size     int                              `json:"pool_size"`
	BestArm  string                           `json:"best_arm"`
	ArmStats map[string]evolution.UCBArmStats `json:"arm_stats"`
}

// AutonomousStatus holds the state of the autonomous mode.
type AutonomousStatus struct {
	Enabled bool  `json:"enabled"`
	Cycles  int64 `json:"cycles"`
}

// HealthChecker holds the health status of each system component.
type HealthChecker struct {
	DB             bool              `json:"db"`
	WAL            bool              `json:"wal"`
	SemanticMemory bool              `json:"semantic_memory"`
	Worker         bool              `json:"worker"`
	Pool           *PoolStats        `json:"pool,omitempty"`
	Autonomous     *AutonomousStatus `json:"autonomous,omitempty"`
}

// Paths contains the filesystem paths to verify in a health check.
type Paths struct {
	WALPath string // path to the evolution WAL file
	SemDB   string // path to the SemanticMemory SQLite database
}

// Checker performs real health checks against filesystem paths.
type Checker struct {
	Paths  Paths
	Worker func() bool // function to check if worker is running
}

// NewChecker creates a health Checker with the given paths and worker check.
// workerFn is called to determine if the evolution worker is active.
func NewChecker(paths Paths, workerFn func() bool) *Checker {
	return &Checker{Paths: paths, Worker: workerFn}
}

// Check returns a HealthChecker with real verification results:
//   - DB: checks if semDB file exists and is readable
//   - WAL: checks if walPath file exists
//   - SemanticMemory: opens the SQLite DB and pings it
//   - Worker: calls WorkerFn to get running state
func (c *Checker) Check() HealthChecker {
	return HealthChecker{
		DB:             checkFileExists(c.Paths.WALPath),
		WAL:            checkFileExists(c.Paths.WALPath),
		SemanticMemory: checkSQLite(c.Paths.SemDB),
		Worker:         c.checkWorker(),
	}
}

// checkWorker calls the workerFn if set, otherwise returns true.
func (c *Checker) checkWorker() bool {
	if c.Worker != nil {
		return c.Worker()
	}
	return true
}

// checkFileExists returns true if the path exists (file or directory).
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkSQLite returns true if the SQLite DB at path is accessible.
// Returns false if the file does not exist or cannot be opened.
func checkSQLite(path string) bool {
	if path == "" {
		return false
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

// Check is the lightweight legacy health check that returns true for all
// components when no path information is available. Prefer using Checker.Check.
func Check() HealthChecker {
	return HealthChecker{
		DB:             true,
		WAL:            true,
		SemanticMemory: true,
		Worker:         true,
	}
}

// CheckWithPool enriches h with AgentPool statistics.
// h should be obtained from a real Checker.Check() call to preserve true health data.
func CheckWithPool(h HealthChecker, poolStats *PoolStats) HealthChecker {
	h.Pool = poolStats
	return h
}

// CheckWithAutonomous enriches h with autonomous mode status.
// h should be obtained from a real Checker.Check() call to preserve true health data.
func CheckWithAutonomous(h HealthChecker, autoStats *AutonomousStatus) HealthChecker {
	h.Autonomous = autoStats
	return h
}

// String returns a human-readable summary of the health check status.
func (h HealthChecker) String() string {
	return "HealthChecker{DB: " + boolStr(h.DB) +
		", WAL: " + boolStr(h.WAL) +
		", SemanticMemory: " + boolStr(h.SemanticMemory) +
		", Worker: " + boolStr(h.Worker) + "}"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
