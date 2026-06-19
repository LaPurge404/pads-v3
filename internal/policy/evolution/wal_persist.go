package evolution

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

type WALStore struct {
	path string
}

func NewWALStore(path string) *WALStore {
	return &WALStore{path: path}
}

func (s *WALStore) Append(entry Entry) error {
	// Créer le répertoire parent si nécessaire
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

func (s *WALStore) Replay() ([]Entry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			entries = append(entries, e)
		}
	}

	return entries, scanner.Err()
}
