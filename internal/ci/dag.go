package ci

import (
    "fmt"
    "sort"
)

// ResolveJobOrder performs a deterministic topological sort using Kahn's algorithm.
func ResolveJobOrder(jobs map[string]Job) ([]Job, error) {
    indegree := make(map[string]int)
    graph := make(map[string][]string)

    for id := range jobs {
        indegree[id] = 0
    }

    ids := make([]string, 0, len(jobs))
    for id := range jobs {
        ids = append(ids, id)
    }
    sort.Strings(ids)

    for _, id := range ids {
        job := jobs[id]
        deps := make([]string, len(job.Needs))
        copy(deps, job.Needs)
        sort.Strings(deps)

        for _, dep := range deps {
            if _, ok := jobs[dep]; !ok {
                return nil, fmt.Errorf("unknown job: %s", dep)
            }
            graph[dep] = append(graph[dep], id)
            indegree[id]++
        }
    }

    queue := make([]string, 0)
    for _, id := range ids {
        if indegree[id] == 0 {
            queue = append(queue, id)
        }
    }

    var result []Job
    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]
        result = append(result, jobs[current])

        neighbors := make([]string, len(graph[current]))
        copy(neighbors, graph[current])
        sort.Strings(neighbors)

        for _, n := range neighbors {
            indegree[n]--
            if indegree[n] == 0 {
                queue = append(queue, n)
            }
        }
    }

    if len(result) != len(jobs) {
        return nil, fmt.Errorf("cycle detected in job graph")
    }

    return result, nil
}
