package agent

import "pads-v3/internal/storage"

// Task represents a unit of work for an agent.
type Task struct {
    Kind   TaskKind // type of task
    Target string   // file path or node ID
    Goal   string   // human-readable description of the expected outcome
}

// Context provides the agent with all necessary information about the task.
type Context struct {
    FilePath    string
    PackagePath string
    NodeID      string
    L2Events    []storage.Event
    L3State     storage.GraphState
}

// Action represents a single step in a plan.
type Action struct {
    Kind    ActionKind // type of action
    Target  string     // file path or command
    Patch   string     // code diff or content to write
    Command []string   // command to execute
}

// Plan is a sequence of actions to achieve a goal.
type Plan struct {
    Steps []Action
}

// Agent is the interface that all agents must implement.
type Agent interface {
    Solve(task Task, ctx Context) (Plan, error)
}
