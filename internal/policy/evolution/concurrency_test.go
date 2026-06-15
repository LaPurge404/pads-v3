package evolution_test

import (
    "fmt"
    "sync"
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestConcurrentEnqueue(t *testing.T) {
    queue, _ := evolution.NewEventQueue(t.TempDir() + "/concurrent.log")
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            queue.Enqueue(evolution.QueueEvent{
                ID:        fmt.Sprintf("%d", id),
                Type:      "evolve",
                Candidate: 50 + id,
                Current:   40,
                Weight:    1.0,
                Mode:      evolution.ModeStable,
            })
        }(i)
    }
    wg.Wait()

    events, _ := queue.LoadAll()
    if len(events) != 10 {
        t.Errorf("expected 10 events, got %d", len(events))
    }
}
