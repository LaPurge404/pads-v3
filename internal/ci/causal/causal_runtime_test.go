package causal

import (
	"testing"

	"pads-v3/internal/event"
)

func TestInstrumentEvents_Basic(t *testing.T) {
	events := []event.CanonicalEvent{
		{Type: "CI_JOB_STARTED", JobID: "test"},
		{Type: "CI_STEP_STARTED", JobID: "test", StepID: "step1"},
		{Type: "CI_CACHE_MISS", JobID: "test", StepID: "step1"},
		{Type: "CI_STEP_FINISHED", JobID: "test", StepID: "step1"},
		{Type: "CI_ARTIFACT", JobID: "test", StepID: "step1"},
		{Type: "CI_JOB_FINISHED", JobID: "test"},
	}

	causalEvents := InstrumentEvents(events)

	if len(causalEvents) != len(events) {
		t.Fatalf("expected %d causal events, got %d", len(events), len(causalEvents))
	}

	// Verify parent chain within the same job
	for i := 1; i < len(causalEvents); i++ {
		if causalEvents[i].JobID == causalEvents[i-1].JobID {
			if causalEvents[i].ParentID != causalEvents[i-1].CausalID {
				t.Errorf("ParentID mismatch at index %d: expected %s, got %s",
					i, causalEvents[i-1].CausalID, causalEvents[i].ParentID)
			}
		}
	}

	// Verify phase assignment
	if causalEvents[0].PhaseID != "JOB_LIFECYCLE" {
		t.Errorf("expected JOB_LIFECYCLE phase for event 0, got %s", causalEvents[0].PhaseID)
	}
	if causalEvents[2].PhaseID != "CACHE_RESOLUTION" {
		t.Errorf("expected CACHE_RESOLUTION phase for event 2, got %s", causalEvents[2].PhaseID)
	}

	t.Logf("Causal instrumentation OK: %d events instrumented", len(causalEvents))
}
