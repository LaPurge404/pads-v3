package causal

import (
    "testing"
)

func TestLocateDivergence_Identical(t *testing.T) {
    events := []CausalEvent{
        {Seq: 1, Type: "CI_JOB_STARTED", JobID: "test", CausalID: "a", ParentID: ""},
        {Seq: 2, Type: "CI_STEP_STARTED", JobID: "test", StepID: "step1", CausalID: "b", ParentID: "a"},
    }
    node := LocateDivergence(events, events)
    if node != nil {
        t.Errorf("expected nil for identical traces, got %+v", node)
    }
}

func TestLocateDivergence_StatusMismatch(t *testing.T) {
    oracle := []CausalEvent{
        {Seq: 1, Type: "CI_CACHE_HIT", JobID: "test", StepID: "step1", Status: "CACHED"},
    }
    replay := []CausalEvent{
        {Seq: 1, Type: "CI_CACHE_MISS", JobID: "test", StepID: "step1", Status: "MISS"},
    }
    node := LocateDivergence(oracle, replay)
    if node == nil {
        t.Fatal("expected divergence node")
    }
    if node.Type != DiffCacheHit {
        t.Errorf("expected DiffCacheHit, got %s", node.Type)
    }
}

func TestLocateDivergence_MissingEvent(t *testing.T) {
    oracle := []CausalEvent{
        {Seq: 1, Type: "CI_JOB_STARTED", JobID: "test"},
        {Seq: 2, Type: "CI_JOB_FINISHED", JobID: "test"},
    }
    replay := []CausalEvent{
        {Seq: 1, Type: "CI_JOB_STARTED", JobID: "test"},
    }
    node := LocateDivergence(oracle, replay)
    if node == nil {
        t.Fatal("expected divergence node")
    }
    if node.Type != DiffMissing {
        t.Errorf("expected DiffMissing, got %s", node.Type)
    }
}
