package causal

import (
    "pads-v3/internal/event"
)

// InstrumentEvents converts a slice of CanonicalEvents into CausalEvents.
func InstrumentEvents(events []event.CanonicalEvent) []CausalEvent {
    var causalEvents []CausalEvent
    var lastCausalID string

    for i, e := range events {
        phase := getPhase(e.Type)
        seq := i // use index as sequence since CanonicalEvent has no Seq
        causalID := ComputeCausalID(e.JobID, e.StepID, phase, seq)

        parentID := ""
        if i > 0 && events[i-1].JobID == e.JobID {
            parentID = lastCausalID
        }

        ce := CausalEvent{
            Seq:      seq,
            Type:     e.Type,
            JobID:    e.JobID,
            StepID:   e.StepID,
            Status:   e.Status,
            Payload:  e.Payload,
            CausalID: causalID,
            ParentID: parentID,
            PhaseID:  phase,
        }

        causalEvents = append(causalEvents, ce)
        lastCausalID = causalID
    }

    return causalEvents
}

// getPhase returns a logical phase name based on the event type.
func getPhase(eventType string) string {
    switch eventType {
    case "CI_JOB_STARTED", "CI_JOB_FINISHED":
        return "JOB_LIFECYCLE"
    case "CI_STEP_STARTED", "CI_STEP_FINISHED":
        return "STEP_EXECUTION"
    case "CI_CACHE_HIT", "CI_CACHE_MISS":
        return "CACHE_RESOLUTION"
    case "CI_ARTIFACT":
        return "ARTIFACT_STORAGE"
    default:
        return "UNKNOWN"
    }
}
