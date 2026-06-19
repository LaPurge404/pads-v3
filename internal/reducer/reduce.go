package reducer

import "pads-v3/internal/storage"

// Reduce applies a single event to the current graph state.
// PURE FUNCTION: no IO, no DB, deterministic.
func Reduce(event storage.Event, currentState map[string]storage.GraphState) map[string]storage.GraphState {
	newState := make(map[string]storage.GraphState)

	switch event.EventType {
	case "SYSTEM_BOOTSTRAP", "GRAPH_INITIAL_BUILT":
		return newState

	case "OS_EXEC_RESULT", "TEST_RESULT":
		var nextState string
		if event.ExitCode == 0 {
			nextState = storage.STATE_STABLE
		} else {
			nextState = storage.STATE_BROKEN
		}
		for _, nodeID := range event.NodeIDs {
			newState[nodeID] = storage.GraphState{
				NodeID:       nodeID,
				State:        nextState,
				LastEventID:  event.EventID,
				LastExitCode: event.ExitCode,
			}
		}
		return newState

	default:
		return newState
	}
}
