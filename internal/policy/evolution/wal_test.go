package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestWAL_Append(t *testing.T) {
    wal := evolution.NewWAL()
    entry := wal.Append(80, 50, 1.0, evolution.ModeStable)
    if entry.CandidateScore != 80 {
        t.Fatalf("expected 80, got %d", entry.CandidateScore)
    }
    if len(entry.Hash) == 0 {
        t.Fatal("hash empty")
    }
}

func TestWAL_LastEntry(t *testing.T) {
    wal := evolution.NewWAL()
    wal.Append(10, 5, 0.5, evolution.ModeBandit)
    last := wal.LastEntry()
    if last == nil || last.CandidateScore != 10 {
        t.Fatal("last entry mismatch")
    }
}

func TestWALStore_PersistAndReplay(t *testing.T) {
    tmp := t.TempDir() + "/wal.log"
    store := evolution.NewWALStore(tmp)

    entry := evolution.Entry{
        CandidateScore: 42,
        CurrentScore:   7,
        Weight:         0.8,
        Mode:           evolution.ModeStable,
    }
    err := store.Append(entry)
    if err != nil {
        t.Fatal(err)
    }

    entries, err := store.Replay()
    if err != nil {
        t.Fatal(err)
    }
    if len(entries) != 1 || entries[0].CandidateScore != 42 {
        t.Fatalf("replay mismatch: %+v", entries)
    }
}

func TestWALBridge_Append(t *testing.T) {
    mem := evolution.NewWAL()
    disk := evolution.NewWALStore(t.TempDir() + "/bridge.log")
    bridge := evolution.NewWALBridge(mem, disk)

    _, err := bridge.Append(100, 90, 1.0, evolution.ModeStable)
    if err != nil {
        t.Fatal(err)
    }
    if mem.LastEntry().CandidateScore != 100 {
        t.Fatal("mem entry missing")
    }
    entries, _ := disk.Replay()
    if len(entries) != 1 {
        t.Fatal("disk entry missing")
    }
}
