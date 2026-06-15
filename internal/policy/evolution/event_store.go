package evolution

import (
    "bufio"
    "encoding/json"
    "os"
    "path/filepath"
)

type EventStore struct {
    path string
}

func NewEventStore(path string) *EventStore {
    return &EventStore{path: path}
}

func (es *EventStore) Append(ev Event) error {
    dir := filepath.Dir(es.path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    f, err := os.OpenFile(es.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    data, err := json.Marshal(ev)
    if err != nil {
        return err
    }
    _, err = f.Write(append(data, '\n'))
    return err
}

func (es *EventStore) LoadAll() ([]Event, error) {
    f, err := os.Open(es.path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var events []Event
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        var ev Event
        if err := json.Unmarshal(scanner.Bytes(), &ev); err == nil {
            events = append(events, ev)
        }
    }
    return events, scanner.Err()
}
