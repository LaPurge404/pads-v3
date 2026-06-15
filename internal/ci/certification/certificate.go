package certification

// Certificate represents a signed proof of determinism for a CI run.
type Certificate struct {
    RunID            string `json:"run_id"`
    WALHash          string `json:"wal_hash"`
    ReplayHash       string `json:"replay_hash"`
    Deterministic    bool   `json:"deterministic"`
    ArtifactHash     string `json:"artifact_hash"`
    SchedulerVersion string `json:"scheduler_version"`
}
