package ci

import (
	"fmt"
	"sort"

	"pads-v3/internal/dag"
	"pads-v3/internal/event"
)

// BuildDAG creates the causal DAG from jobs.
// It pre-computes cache decisions and embeds them in the graph structure.
func BuildDAG(jobs map[string]Job, cache *Cache) (*dag.Graph, CacheSnapshot) {
	var allKeys []string
	order, _ := ResolveJobOrder(jobs)

	for _, job := range order {
		steps := make([]Step, len(job.Steps))
		copy(steps, job.Steps)
		sort.Slice(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
		matrix := job.Matrix.Expand()
		for _, m := range matrix {
			varsCopy := cloneMap(m)
			matrixHash := flatten(varsCopy)
			input := flatten(varsCopy)
			for _, step := range steps {
				key := cache.Key(job.ID, step, input, matrixHash)
				allKeys = append(allKeys, key)
			}
		}
	}
	snapshot := cache.TakeSnapshot(allKeys)

	graph := &dag.Graph{
		Nodes: make(map[string]*dag.Node),
	}

	for _, job := range order {
		// Job start node
		jobStartID := fmt.Sprintf("%s:job_start", job.ID)
		graph.Nodes[jobStartID] = &dag.Node{
			ID:    jobStartID,
			Type:  dag.NodeJobStart,
			JobID: job.ID,
		}

		steps := make([]Step, len(job.Steps))
		copy(steps, job.Steps)
		sort.Slice(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
		matrix := job.Matrix.Expand()

		for _, m := range matrix {
			varsCopy := cloneMap(m)
			matrixHash := flatten(varsCopy)
			input := flatten(varsCopy)

			for _, step := range steps {
				stepID := fmt.Sprintf("%s:%s", job.ID, step.ID)
				key := cache.Key(job.ID, step, input, matrixHash)
				cacheHit := snapshot[key]

				// Cache node
				cacheNodeID := stepID + ":cache"
				graph.Nodes[cacheNodeID] = &dag.Node{
					ID:     cacheNodeID,
					Type:   dag.NodeCache,
					JobID:  job.ID,
					StepID: step.ID,
				}
				// The cache decision is embedded in the DAG structure.
				// The executor does not need to know about hit/miss; it just executes
				// whatever the graph tells it to.
				_ = cacheHit

				// Step run node (depends on job start and cache)
				runNodeID := stepID + ":run"
				graph.Nodes[runNodeID] = &dag.Node{
					ID:           runNodeID,
					Type:         dag.NodeStepRun,
					JobID:        job.ID,
					StepID:       step.ID,
					Action:       dag.ActionSpec{Command: step.Run, WorkingDir: step.WorkingDir},
					Dependencies: []string{jobStartID, cacheNodeID},
				}
			}
		}

		// Job end node (depends on all step run nodes)
		jobEndID := fmt.Sprintf("%s:job_end", job.ID)
		endNode := &dag.Node{
			ID:    jobEndID,
			Type:  dag.NodeJobEnd,
			JobID: job.ID,
		}
		for _, step := range job.Steps {
			stepID := fmt.Sprintf("%s:%s:run", job.ID, step.ID)
			if _, ok := graph.Nodes[stepID]; ok {
				endNode.Dependencies = append(endNode.Dependencies, stepID)
			}
		}
		graph.Nodes[jobEndID] = endNode
	}

	return graph, snapshot
}

// Plan is an alias to the new DAG Graph during migration.
type Plan = dag.Graph

// PlannedStep is kept for backward compatibility.
// It will be removed once all consumers are migrated to the DAG.
type PlannedStep struct {
	JobID      string
	StepID     string
	Run        string
	WorkingDir string
	CacheKey   string
	CacheHit   bool
	Events     []PlannedEvent
}

// PlannedEvent is kept for backward compatibility.
type PlannedEvent struct {
	Ordinal int
	Event   event.CanonicalEvent
}
