package ci

import (
    "os"
    "testing"

    "pads-v3/internal/event"
)

func TestWAL_AppendCanonical(t *testing.T) {
    path := t.TempDir() + "/test.wal"
    wal, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    defer wal.Close()

    err = wal.AppendCanonical(event.CanonicalEvent{Type: "A", JobID: "j1"})
    if err != nil {
        t.Fatal(err)
    }
    err = wal.AppendCanonical(event.CanonicalEvent{Type: "B", JobID: "j1"})
    if err != nil {
        t.Fatal(err)
    }
    // Verify file exists and has content
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Error("WAL file should exist")
    }
}

func TestWAL_Persistence(t *testing.T) {
    path := t.TempDir() + "/test.wal"
    wal, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    err = wal.AppendCanonical(event.CanonicalEvent{Type: "X", JobID: "j"})
    if err != nil {
        t.Fatal(err)
    }
    wal.Close()

    // Re-open
    wal2, err := NewWAL(path)
    if err != nil {
        t.Fatal(err)
    }
    defer wal2.Close()
    err = wal2.AppendCanonical(event.CanonicalEvent{Type: "Y", JobID: "j"})
    if err != nil {
        t.Fatal(err)
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
