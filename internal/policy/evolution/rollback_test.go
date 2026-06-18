package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestRollbackManager_NoRollbackWhenStable(t *testing.T) {
    wal := evolution.NewWAL("")
    detector := evolution.NewAntiCollapseDetector(5, 10.0)
    detector.Add(10)
    detector.Add(12)

    rm := evolution.NewRollbackManager(wal, detector)
    _, rolledBack := rm.RollbackIfUnstable()
    if rolledBack {
        t.Fatal("should not rollback")
    }
}

func TestRollbackManager_RollbackWhenOscillating(t *testing.T) {
    wal := evolution.NewWAL("")
    detector := evolution.NewAntiCollapseDetector(3, 1.0)
    detector.Add(10)
    detector.Add(100)
    detector.Add(10) // oscillation

    rm := evolution.NewRollbackManager(wal, detector)
    entry, rolledBack := rm.RollbackIfUnstable()
    if !rolledBack {
        t.Fatal("expected rollback")
    }
    // entry peut être nil car WAL vide, mais le rollback doit être signalé
    _ = entry
}
