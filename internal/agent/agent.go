package agent

import (
	"os"
	"strings"

	"pads-v3/internal/semantic/memory"
	"pads-v3/internal/storage"
)

// Task represents a unit of work for an agent.
type Task struct {
	Kind   TaskKind // type of task
	Target string   // file path or node ID
	Goal   string   // human-readable description of the expected outcome
}

// Context provides the agent with all necessary information about the task.
type Context struct {
	FilePath      string
	PackagePath   string
	NodeID        string
	L2Events      []storage.Event
	L3State       storage.GraphState
	SemMem        *memory.SemanticMemory // optional semantic memory for rich context
	SourceContent string                 // optional source code content (injected into prompt)
}

// Enrich populates ctx.SourceContent by reading the first maxLines of targetFile
// and ctx.SemMem.QueryContext if SemMem is set. Call this at call sites before
// passing Context to Solve / BuildPromptForStrategy.
func (ctx *Context) Enrich(targetFile string, maxLines int) {
	if ctx.SourceContent == "" && targetFile != "" {
		if lines, err := readFileLines(targetFile, maxLines); err == nil {
			ctx.SourceContent = lines
		}
	}
	if ctx.SemMem != nil && targetFile != "" {
		if q := ctx.SemMem.QueryContext(targetFile); q != "" {
			ctx.SourceContent += "\n\n" + q
		}
	}
}

// readFileLines reads at most n lines from path. Returns empty string on error.
func readFileLines(path string, n int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n"), nil
	}
	return strings.Join(lines[:n], "\n"), nil
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