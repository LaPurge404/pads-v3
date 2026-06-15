package evolution

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "time"
)

type Entry struct {
    CandidateScore int
    CurrentScore   int
    Weight         float64
    Mode           Mode
    Timestamp      time.Time
    Hash           string
    PrevHash       string
}

type WAL struct {
    entries []Entry
}

func NewWAL() *WAL {
    return &WAL{}
}

func (w *WAL) Append(candidate, current int, weight float64, mode Mode) Entry {
    var prevHash string
    if len(w.entries) > 0 {
        prevHash = w.entries[len(w.entries)-1].Hash
    }
    entry := Entry{
        CandidateScore: candidate,
        CurrentScore:   current,
        Weight:         weight,
        Mode:           mode,
        Timestamp:      time.Now(),
        PrevHash:       prevHash,
    }
    entry.Hash = computeHash(entry)
    w.entries = append(w.entries, entry)
    return entry
}

func computeHash(e Entry) string {
    type entryWithoutHash struct {
        CandidateScore int
        CurrentScore   int
        Weight         float64
        Mode           Mode
        Timestamp      time.Time
        PrevHash       string
    }
    tmp := entryWithoutHash{
        CandidateScore: e.CandidateScore,
        CurrentScore:   e.CurrentScore,
        Weight:         e.Weight,
        Mode:           e.Mode,
        Timestamp:      e.Timestamp,
        PrevHash:       e.PrevHash,
    }
    data, _ := json.Marshal(tmp)
    hash := sha256.Sum256(data)
    return hex.EncodeToString(hash[:])
}

func (w *WAL) LastEntry() *Entry {
    if len(w.entries) == 0 {
        return nil
    }
    return &w.entries[len(w.entries)-1]
}

func (w *WAL) Snapshot() *Entry {
    return w.LastEntry()
}
