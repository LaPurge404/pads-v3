package dag

import (
    "fmt"
    "sort"
)

// ResolveOrder returns a deterministic topological ordering of the nodes.
func (g *Graph) ResolveOrder() ([]*Node, error) {
    indegree := make(map[string]int)
    for id := range g.Nodes {
        indegree[id] = 0
    }
    for _, n := range g.Nodes {
        for _, dep := range n.Dependencies {
            if _, ok := g.Nodes[dep]; !ok {
                return nil, fmt.Errorf("unknown dependency: %s", dep)
            }
            indegree[n.ID]++
        }
    }

    ids := make([]string, 0, len(g.Nodes))
    for id := range g.Nodes {
        ids = append(ids, id)
    }
    sort.Strings(ids)

    var ready []string
    for _, id := range ids {
        if indegree[id] == 0 {
            ready = append(ready, id)
        }
    }

    var order []*Node
    for len(ready) > 0 {
        current := ready[0]
        ready = ready[1:]
        order = append(order, g.Nodes[current])

        for _, n := range g.Nodes {
            for _, dep := range n.Dependencies {
                if dep == current {
                    indegree[n.ID]--
                    if indegree[n.ID] == 0 {
                        ready = append(ready, n.ID)
                        sort.Strings(ready)
                    }
                }
            }
        }
    }

    if len(order) != len(g.Nodes) {
        return nil, fmt.Errorf("cycle detected in DAG")
    }

    return order, nil
}
