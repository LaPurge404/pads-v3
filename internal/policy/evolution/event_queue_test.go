package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestEventQueue_EnqueueAndLoad(t *testing.T) {
    q, err := evolution.NewEventQueue(t.TempDir() + "/queue.log")
    if err != nil {
        t.Fatal(err)
    }
    e := evolution.QueueEvent{
        ID:        "1",
        Type:      "evolve",
        Candidate: 99,
        Current:   88,
        Weight:    0.9,
        Mode:      evolution.ModeBandit,
    }
    err = q.Enqueue(e)
    if err != nil {
        t.Fatal(err)
    }
    events, err := q.LoadAll()
    if err != nil {
        t.Fatal(err)
    }
    if len(events) != 1 || events[0].Candidate != 99 {
        t.Fatalf("unexpected events: %+v", events)
    }
}
