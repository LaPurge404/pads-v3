package ci

import (
    "encoding/json"
    "os"
    "sync"
)

// EventRecord represents a durable CI lifecycle event in the WAL.
type EventRecord struct {
    Seq     int64  `json:"seq"`
    Type    string `json:"type"`
    JobID   string `json:"job_id"`
    StepID  string `json:"step_id"`
    Status  string `json:"status"`
    Payload string `json:"payload"`
}

// WAL is an append-only event log stored on disk.
type WAL struct {
    mu   sync.Mutex
    file *os.File
    seq  int64
}

// NewWAL opens or creates a WAL file at the given path.
func NewWAL(path string) (*WAL, error) {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
    if err != nil {
        return nil, err
    }
    // Récupérer le dernier seq en lisant le fichier
    var lastSeq int64
    dec := json.NewDecoder(f)
    for {
        var rec EventRecord
        if err := dec.Decode(&rec); err != nil {
            break
        }
        if rec.Seq > lastSeq {
            lastSeq = rec.Seq
        }
    }
    return &WAL{file: f, seq: lastSeq}, nil
}

// Append ajoute un événement au WAL de manière thread-safe et retourne le Seq attribué.
func (w *WAL) Append(e EventRecord) (int64, error) {
    w.mu.Lock()
    defer w.mu.Unlock()

    w.seq++
    e.Seq = w.seq

    b, err := json.Marshal(e)
    if err != nil {
        return 0, err
    }
    b = append(b, '\n')
    if _, err := w.file.Write(b); err != nil {
        return 0, err
    }
    // Force flush pour durabilité
    if err := w.file.Sync(); err != nil {
        return 0, err
    }
    return w.seq, nil
}

// Close ferme le fichier WAL.
func (w *WAL) Close() error {
    return w.file.Close()
}
