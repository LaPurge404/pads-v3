package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestEventStore_AppendAndLoad(t *testing.T) {
    store := evolution.NewEventStore(t.TempDir() + "/events.log")
    ev := evolution.Event{
        Sequence:       1,
        CandidateScore: 88,
        CurrentScore:   44,
        Weight:         1.0,
        Mode:           evolution.ModeStable,
        BanditSeed:     0,
    }
    err := store.Append(ev)
    if err != nil {
        t.Fatal(err)
    }

    events, err := store.LoadAll()
    if err != nil {
        t.Fatal(err)
    }
    if len(events) != 1 || events[0].CandidateScore != 88 {
        t.Fatalf("unexpected events: %+v", events)
    }
}
