package evolution

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// QueueEvent est l'unité soumise à la file d'attente.
type QueueEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Candidate int               `json:"candidate"`
	Current   int               `json:"current"`
	Weight    float64           `json:"weight"`
	Mode      Mode              `json:"mode"`
	Metadata  map[string]string `json:"metadata,omitempty"` // données additionnelles (stratégie, confidence, etc.)
}

// EventQueue est une file persistée (append‑only) thread‑safe.
type EventQueue struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func NewEventQueue(path string) (*EventQueue, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("ouverture queue %s: %w", path, err)
	}
	return &EventQueue{file: f, path: path}, nil
}

// Close ferme le fichier sous-jacent.
func (q *EventQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.file != nil {
		return q.file.Close()
	}
	return nil
}

// Enqueue écrit immédiatement l'événement sur disque et retourne une erreur si l'écriture échoue.
func (q *EventQueue) Enqueue(e QueueEvent) error {
q.mu.Lock()
defer q.mu.Unlock()

data, err := json.Marshal(e)
if err != nil {
return err
}
_, err = q.file.Write(append(data, '\n'))
return err
}

// LoadAll lit tous les événements de la file (crash recovery).
func (q *EventQueue) LoadAll() ([]QueueEvent, error) {
	f, err := os.Open(q.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []QueueEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e QueueEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			events = append(events, e)
		}
	}
	return events, scanner.Err()
}

// ReadFrom lit les événements à partir de l'offset donné (en octets dans le fichier).
// Retourne les événements, le nouvel offset, et une erreur.
func (q *EventQueue) ReadFrom(offset int64) ([]QueueEvent, int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	f, err := os.Open(q.path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

	// Se positionner à l'offset
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	var events []QueueEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e QueueEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			events = append(events, e)
		}
	}
	newOffset, _ := f.Seek(0, io.SeekCurrent)
	return events, newOffset, scanner.Err()
}
