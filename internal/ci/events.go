package ci

// EventType categorises CI lifecycle events.
type EventType string

const (
EventJobStarted  EventType = "CI_JOB_STARTED"
EventJobFinished EventType = "CI_JOB_FINISHED"
EventStepStart   EventType = "CI_STEP_STARTED"
EventStepEnd     EventType = "CI_STEP_FINISHED"
EventCacheHit    EventType = "CI_CACHE_HIT"
EventCacheMiss   EventType = "CI_CACHE_MISS"
)

// Event is a CI lifecycle event.
type Event struct {
Type   EventType
JobID  string
StepID string
Data   string
}
