package ci

import (
    "os"
    "testing"
)

func TestWAL_AppendAndSequence(t *testing.T) {
    path := t.TempDir() + "/test.wal"
    wal, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    defer wal.Close()

    seq1, err := wal.Append(EventRecord{Type: "A", JobID: "j1"})
    if err != nil {
        t.Fatal(err)
    }
    seq2, err := wal.Append(EventRecord{Type: "B", JobID: "j1"})
    if err != nil {
        t.Fatal(err)
    }
    if seq1 != 1 || seq2 != 2 {
        t.Errorf("expected seq 1 and 2, got %d and %d", seq1, seq2)
    }
}

func TestWAL_Persistence(t *testing.T) {
    path := t.TempDir() + "/test.wal"
    wal, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    _, err = wal.Append(EventRecord{Type: "X", JobID: "j"})
    if err != nil {
        t.Fatal(err)
    }
    wal.Close()

    // Re-open and check that sequence continues
    wal2, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    defer wal2.Close()
    seq, err := wal2.Append(EventRecord{Type: "Y", JobID: "j"})
    if err != nil {
        t.Fatal(err)
    }
    if seq != 2 {
        t.Errorf("expected seq 2 after reopen, got %d", seq)
    }
}

func TestWAL_FileExists(t *testing.T) {
    path := t.TempDir() + "/newwal.wal"
    wal, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    wal.Close()
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Error("WAL file should exist after creation")
    }
}
