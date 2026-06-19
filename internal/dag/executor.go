package dag

import (
	"fmt"
	"os/exec"
	"sort"
	"sync"

	"pads-v3/internal/event"
)

// Executor runs the DAG nodes with a single-threaded deterministic scheduler
// and a concurrent worker pool for execution.
type Executor struct {
	Graph       *Graph
	MaxParallel int
}

// Run executes the DAG and returns the events in topological order.
func (e *Executor) Run() ([]event.CanonicalEvent, error) {
	if e.MaxParallel <= 0 {
		e.MaxParallel = 4
	}

	// 1. Build adjacency list and indegree (scheduler state, NOT mutated by workers)
	adj := make(map[string][]string)
	indegree := make(map[string]int)

	for id := range e.Graph.Nodes {
		indegree[id] = 0
	}
	for _, n := range e.Graph.Nodes {
		for _, dep := range n.Dependencies {
			if _, ok := e.Graph.Nodes[dep]; !ok {
				return nil, fmt.Errorf("unknown dependency: %s", dep)
			}
			adj[dep] = append(adj[dep], n.ID)
			indegree[n.ID]++
		}
	}

	for id := range adj {
		sort.Strings(adj[id])
	}

	// 2. Initial ready queue
	ready := make([]*Node, 0)
	for _, n := range e.Graph.Nodes {
		if indegree[n.ID] == 0 {
			ready = append(ready, n)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].ID < ready[j].ID })

	// 3. Single-threaded scheduler loop
	var allEvents []event.CanonicalEvent
	sem := make(chan struct{}, e.MaxParallel)

	for len(ready) > 0 {
		batchSize := len(ready)
		if batchSize > e.MaxParallel {
			batchSize = e.MaxParallel
		}
		batch := ready[:batchSize]
		ready = ready[batchSize:]

		var wg sync.WaitGroup
		results := make(map[string][]event.CanonicalEvent)
		var resMu sync.Mutex

		for _, node := range batch {
			wg.Add(1)
			go func(n *Node) {
				defer wg.Done()
				defer func() { <-sem }()
				sem <- struct{}{}

				events := executeNode(n)

				resMu.Lock()
				results[n.ID] = events
				resMu.Unlock()
			}(node)
		}
		wg.Wait()

		// Collect events in deterministic batch order
		for _, node := range batch {
			if evs, ok := results[node.ID]; ok {
				allEvents = append(allEvents, evs...)
			}
		}

		// Update scheduler state (single-threaded)
		for _, node := range batch {
			for _, depID := range adj[node.ID] {
				indegree[depID]--
				if indegree[depID] == 0 {
					ready = append(ready, e.Graph.Nodes[depID])
				}
			}
		}
		sort.Slice(ready, func(i, j int) bool { return ready[i].ID < ready[j].ID })
	}

	return allEvents, nil
}

// executeNode runs a single node and returns the CanonicalEvents it produces.
func executeNode(n *Node) []event.CanonicalEvent {
	var events []event.CanonicalEvent

	switch n.Type {
	case NodeJobStart:
		events = append(events, event.CanonicalEvent{
			NodeID: n.ID,
			Type:   "CI_JOB_STARTED",
			JobID:  n.JobID,
			Status: "RUNNING",
		})
	case NodeJobEnd:
		events = append(events, event.CanonicalEvent{
			NodeID: n.ID,
			Type:   "CI_JOB_FINISHED",
			JobID:  n.JobID,
			Status: "SUCCESS",
		})
	case NodeCache:
		// Cache decisions are embedded in the DAG structure.
	case NodeStepRun:
		events = append(events, event.CanonicalEvent{
			NodeID: n.ID,
			Type:   "CI_STEP_STARTED",
			JobID:  n.JobID,
			StepID: n.StepID,
			Status: "RUNNING",
		})
		cmd := exec.Command("sh", "-c", n.Action.Command)
		if n.Action.WorkingDir != "" {
			cmd.Dir = n.Action.WorkingDir
		}
		out, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		if exitCode == 0 {
			events = append(events, event.CanonicalEvent{
				NodeID:  n.ID,
				Type:    "CI_STEP_FINISHED",
				JobID:   n.JobID,
				StepID:  n.StepID,
				Status:  "SUCCESS",
				Payload: string(out),
			})
			events = append(events, event.CanonicalEvent{
				NodeID:  n.ID,
				Type:    "CI_ARTIFACT",
				JobID:   n.JobID,
				StepID:  n.StepID,
				Status:  "CREATED",
				Payload: fmt.Sprintf(`{"output":"%s"}`, string(out)),
			})
		} else {
			events = append(events, event.CanonicalEvent{
				NodeID:  n.ID,
				Type:    "CI_STEP_FINISHED",
				JobID:   n.JobID,
				StepID:  n.StepID,
				Status:  "FAILED",
				Payload: string(out),
			})
		}
	}
	return events
}
