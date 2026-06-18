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
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// EventQueue est une file persistée (append‑only) thread‑safe.
// Elle maintient un seul descripteur de fichier ouvert pour les lectures
// et les écritures, évitant les ouvertures/fermetures répétées.
//
// L'offset de lecture est géré INTERNE par ReadFrom() et n'est jamais
// modifié par Enqueue(). Cela garantit que les événements viennent d'être
// écrits par Enqueue() sont immédiatement lisibles par ReadFrom().
type EventQueue struct {
	mu    sync.Mutex
	file  *os.File
	path  string
	offset int64 // position de lecture suivante (en octets depuis le début du fichier)
}

func NewEventQueue(path string) (*EventQueue, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("ouverture queue %s: %w", path, err)
	}
	return &EventQueue{file: f, path: path, offset: 0}, nil
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

// Enqueue écrit immédiatement l'événement sur disque.
// O_APPEND guarantees atomic appends. L'offset de lecture n'est PAS modifié
// (ReadFrom lit toujours depuis la position logique courante).
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

// LoadAll lit tous les événements de la file (crash recovery).
// Ouvre un nouveau descripteur pour ne pas perturber l'offset de lecture.
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
		}
	}
	return events, scanner.Err()
}

// ReadFrom lit les événements à partir de la position offset actuelle.
// Elle réutilise le descripteur existant. Après lecture, offset est avancé
// à la fin du fichier pour que le prochain appel ne relise pas les données
// déjà obtenues. Les doublons sont filtrés en amont par le worker via la
// map processed.
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
		}
	}

	// Avancer offset à la fin = fin de ce que le scanner a lu
	newOffset, err := q.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return events, err
	}
	q.offset = newOffset
	return events, scanner.Err()
}

// ReadOffset retourne l'offset de lecture actuel (utile pour le debugging).
func (q *EventQueue) ReadOffset() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.offset
}

// Size retourne la taille actuelle du fichier en octets.
func (q *EventQueue) Size() (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	stat, err := q.file.Stat()
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}