package evolution

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// QueueEvent is the unit submitted to the event queue.
type QueueEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Candidate int               `json:"candidate"`
	Current   int               `json:"current"`
	Weight    float64           `json:"weight"`
	Mode      Mode              `json:"mode"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// EventQueue is a thread-safe, persistent (append-only) queue.
// It keeps a single file descriptor open for both reads and writes,
// avoiding repeated open/close calls.
//
// The read offset is managed INTERNALLY by ReadFrom() and is never
// modified by Enqueue(). This guarantees that events just written by
// Enqueue() are immediately readable by ReadFrom().
type EventQueue struct {
	mu     sync.Mutex
	file   *os.File
	path   string
	offset int64 // next read position in bytes from the start of the file
}

func NewEventQueue(path string) (*EventQueue, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("ouverture queue %s: %w", path, err)
	}
	return &EventQueue{file: f, path: path, offset: 0}, nil
}

// Close closes the underlying file descriptor.
func (q *EventQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.file != nil {
		return q.file.Close()
	}
	return nil
}

// Enqueue writes the event to disk immediately.
// O_APPEND guarantees atomic appends. The read offset is NOT modified
// (ReadFrom always reads from the current logical position).
func (q *EventQueue) Enqueue(e QueueEvent) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	// O_APPEND already set at OpenFile: writes always go to EOF
	_, err = q.file.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	return q.file.Sync()
}

// LoadAll reads all events from the queue (crash recovery).
// Opens a new file descriptor to avoid disturbing the current read offset.
func (q *EventQueue) LoadAll() ([]QueueEvent, error) {
	q.mu.Lock()
	f, err := os.Open(q.path)
	if err != nil {
		q.mu.Unlock()
		return nil, err
	}
	q.mu.Unlock()
	defer f.Close()

	var events []QueueEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e QueueEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			events = append(events, e)
		} else {
			slog.Warn("event_queue: skipped malformed JSON line", "error", err)
		}
	}
	return events, scanner.Err()
}

// ReadFrom reads events starting from the current offset position.
// It reuses the existing file descriptor. After reading, offset is advanced
// to EOF so the next call does not re-read already-obtained data.
// Duplicates are filtered upstream by the worker using the processed map.
func (q *EventQueue) ReadFrom() ([]QueueEvent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.file == nil {
		return nil, fmt.Errorf("queue file is not open")
	}

	if _, err := q.file.Seek(q.offset, io.SeekStart); err != nil {
		return nil, err
	}

	var events []QueueEvent
	scanner := bufio.NewScanner(q.file)
	for scanner.Scan() {
		var e QueueEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			events = append(events, e)
		} else {
			slog.Warn("event_queue: skipped malformed JSON line", "error", err)
		}
	}

	// Advance offset to EOF = end of what the scanner just read
	newOffset, err := q.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return events, err
	}
	q.offset = newOffset
	return events, scanner.Err()
}

// ReadOffset returns the current read offset (useful for debugging).
func (q *EventQueue) ReadOffset() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.offset
}

// Size returns the current file size in bytes.
func (q *EventQueue) Size() (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	stat, err := q.file.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}

// Path returns the WAL file path used by this queue.
func (q *EventQueue) Path() string {
	return q.path
}
