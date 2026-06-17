package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// CodeAgent is an Agent that uses an LLM to generate code modifications.
type CodeAgent struct {
	llm          LLMClient
	executor     *Executor
	maxRetries   int
	minConfidence float64
}

// NewCodeAgent creates a CodeAgent with the given LLM client.
func NewCodeAgent(llm LLMClient) *CodeAgent {
	return &CodeAgent{
		llm:          llm,
		executor:     &Executor{DryRun: false},
		maxRetries:   3,
		minConfidence: 0.6,
	}
}

// Solve generates a plan to fix the given task using the LLM.
func (a *CodeAgent) Solve(task Task, ctx Context) (Plan, error) {
	if task.Kind != TaskFixBroken {
		return Plan{}, fmt.Errorf("CodeAgent only handles TaskFixBroken, got %s", task.Kind)
	}

	// Build the prompt for the LLM
	prompt := a.buildPrompt(task, ctx)

	// Query the LLM
	resp, err := a.llm.GenerateCode(context.Background(), prompt)
	if err != nil {
		return Plan{}, fmt.Errorf("LLM error: %w", err)
	}

	log.Printf("CodeAgent: LLM confidence=%.2f, warnings=%v", resp.Confidence, resp.Warnings)

	// Filter based on confidence
	if resp.Confidence < a.minConfidence {
		log.Printf("CodeAgent: confidence %.2f below threshold %.2f, skipping",
			resp.Confidence, a.minConfidence)
		return Plan{}, fmt.Errorf("low confidence: %.2f", resp.Confidence)
	}

	// Build the plan from the LLM response
	plan := a.buildPlan(task, resp)

	return plan, nil
}

// buildPrompt creates a CodePrompt from the task and context.
func (a *CodeAgent) buildPrompt(task Task, ctx Context) CodePrompt {
	var constraints strings.Builder
	constraints.WriteString("- Output only the complete, working code\n")
	constraints.WriteString("- Follow existing code style and conventions\n")
	constraints.WriteString("- Include appropriate tests if applicable\n")
	constraints.WriteString("- Do not add unrelated changes\n")

	var context strings.Builder
	if ctx.FilePath != "" {
		context.WriteString(fmt.Sprintf("File: %s\n", ctx.FilePath))
	}
	if ctx.PackagePath != "" {
		context.WriteString(fmt.Sprintf("Package: %s\n", ctx.PackagePath))
	}
	if ctx.NodeID != "" {
		context.WriteString(fmt.Sprintf("Node ID: %s\n", ctx.NodeID))
	}

	// Add recent L2 events if available
	if len(ctx.L2Events) > 0 {
		context.WriteString("\nRecent events:\n")
		for i, ev := range ctx.L2Events {
			if i >= 5 { // Limit context size
				break
			}
			context.WriteString(fmt.Sprintf("  - %s: %s\n", ev.EventType, ev.Payload))
		}
	}

	return CodePrompt{
		Task:       task.Goal,
		FilePath:   task.Target,
		Language:   detectLanguage(task.Target),
		Context:    context.String(),
		Constraints: constraints.String(),
	}
}

// buildPlan converts an LLM response into an executable Plan.
func (a *CodeAgent) buildPlan(task Task, resp *CodeResponse) Plan {
	var steps []Action

	// The patch can be either a full file content or a diff
	if isDiff(resp.Patch) {
		// For diffs, we'd need a patch application step
		// For now, treat as full file replacement
		steps = append(steps, Action{
			Kind:   ActionWriteFile,
			Target: task.Target,
			Patch:  stripDiffMarkers(resp.Patch),
		})
	} else {
		steps = append(steps, Action{
			Kind:   ActionWriteFile,
			Target: task.Target,
			Patch:  resp.Patch,
		})
	}

	// Add a test run after the fix
	steps = append(steps, Action{
		Kind:    ActionRunCommand,
		Target:  "go test -run " + testNameForFile(task.Target) + " ./...",
		Command: []string{"bash", "-c", "cd " + dirForFile(task.Target) + " && go test ./..."},
	})

	return Plan{Steps: steps}
}

// detectLanguage guesses the programming language from file extension.
func detectLanguage(filePath string) string {
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return "go"
	case strings.HasSuffix(filePath, ".py"):
		return "python"
	case strings.HasSuffix(filePath, ".js"):
		return "javascript"
	case strings.HasSuffix(filePath, ".ts"):
		return "typescript"
	default:
		return "unknown"
	}
}

// isDiff checks if the patch looks like a diff.
func isDiff(patch string) bool {
	return strings.HasPrefix(patch, "---") ||
		strings.HasPrefix(patch, "diff ") ||
		strings.HasPrefix(patch, "+++")
}

// stripDiffMarkers removes diff headers from a patch to get clean code.
func stripDiffMarkers(patch string) string {
	lines := strings.Split(patch, "\n")
	var result []string
	inDiff := false
	for _, line := range lines {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "@@") {
			inDiff = true
			continue
		}
		if !inDiff || (!strings.HasPrefix(line, "-") && !strings.HasPrefix(line, " ")) {
			inDiff = false
		}
		if !inDiff && !strings.HasPrefix(line, "-") {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// dirForFile returns the directory containing a file.
func dirForFile(filePath string) string {
	idx := strings.LastIndex(filePath, "/")
	if idx < 0 {
		return "."
	}
	return filePath[:idx]
}

// testNameForFile generates a test name pattern for a source file.
func testNameForFile(filePath string) string {
	// e.g., add.go -> TestAdd
	base := strings.TrimSuffix(filePath, ".go")
	base = strings.TrimSuffix(base, ".py")
	return "Test" + strings.Title(base)
}