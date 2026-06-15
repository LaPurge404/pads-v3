package evolution

type RollbackManager struct {
    wal      *WAL
    detector *AntiCollapseDetector
}

func NewRollbackManager(wal *WAL, detector *AntiCollapseDetector) *RollbackManager {
    return &RollbackManager{
        wal:      wal,
        detector: detector,
    }
}

func (r *RollbackManager) IsUnstable() bool {
    return !r.detector.IsStable() || r.detector.IsOscillating()
}

func (r *RollbackManager) RestoreStableState() *Entry {
    return r.wal.Snapshot()
}

func (r *RollbackManager) RollbackIfUnstable() (*Entry, bool) {
    if r.IsUnstable() {
        return r.RestoreStableState(), true
    }
    return nil, false
}
