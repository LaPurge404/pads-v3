package evolution

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
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
	file    *os.File
	path    string
}

func NewWAL(path string) *WAL {
	wal := &WAL{
		entries: make([]Entry, 0),
		path:    path,
	}

	// Try to load existing entries from disk
	if path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644); err == nil {
			wal.file = f
			wal.loadFromDisk()
		}
	}

	return wal
}

func (w *WAL) loadFromDisk() {
	if w.file == nil {
		return
	}

	// Seek to beginning to read all entries
	if _, err := w.file.Seek(0, 0); err != nil {
		return
	}

	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err == nil {
			w.entries = append(w.entries, entry)
		}
	}
}

func (w *WAL) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *WAL) Append(candidate, current int, weight float64, mode Mode) (Entry, error) {
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

	// Persist immediately to disk
	if err := w.flushEntry(entry); err != nil {
		return entry, err
	}

	return entry, nil
}

func (w *WAL) flushEntry(entry Entry) error {
	if w.file == nil {
		return nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Append to file with newline
	if _, err := w.file.Write(append(data, '\n')); err != nil {
		slog.Error("WAL write error", "err", err)
		return err
	}
	// Sync to ensure it's written to disk
	if err := w.file.Sync(); err != nil {
		slog.Error("WAL sync error", "err", err)
		return err
	}
	return nil
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

// Entries returns a copy of all entries (for testing/debugging).
func (w *WAL) Entries() []Entry {
	result := make([]Entry, len(w.entries))
	copy(result, w.entries)
	return result
}