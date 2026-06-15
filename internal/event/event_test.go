package event

import (
    "encoding/json"
    "testing"
)

func TestCanonicalEvent_JSON(t *testing.T) {
    e := CanonicalEvent{Type: "CI_JOB_STARTED", JobID: "j1"}
    b, err := json.Marshal(e)
    if err != nil {
        t.Fatal(err)
    }
    var e2 CanonicalEvent
    if err := json.Unmarshal(b, &e2); err != nil {
        t.Fatal(err)
    }
    if e2.Type != "CI_JOB_STARTED" || e2.JobID != "j1" {
        t.Errorf("round-trip failed: %+v", e2)
    }
}

func TestCanonicalEvent_Equality(t *testing.T) {
    a := CanonicalEvent{Type: "A", JobID: "j"}
    b := CanonicalEvent{Type: "A", JobID: "j"}
    if a != b {
        t.Errorf("expected equality")
    }
}
