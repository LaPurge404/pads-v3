package agent

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"unicode"
)

// CodeAgent is an Agent that uses an LLM to generate code modifications.
type CodeAgent struct {
	llm           LLMClient
	executor      *Executor
	maxRetries    int
	minConfidence float64
}

// NewCodeAgent creates a CodeAgent with the given LLM client.
func NewCodeAgent(llm LLMClient) *CodeAgent {
	return &CodeAgent{
		llm:           llm,
		executor:      &Executor{DryRun: false},
		maxRetries:    3,
		minConfidence: 0.6,
	}
}

// NewCodeAgentDefault creates a CodeAgent with the default Nvidia LLM client.
// This is the recommended constructor when no specific provider is needed.
func NewCodeAgentDefault() *CodeAgent {
	return NewCodeAgent(NewDefaultLLMClient())
}

// MinConfidence returns the minimum confidence threshold for this agent.
func (a *CodeAgent) MinConfidence() float64 {
	return a.minConfidence
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

	slog.Info("CodeAgent: LLM response", "confidence", resp.Confidence, "warnings", resp.Warnings)

	// Filter based on confidence
	if resp.Confidence < a.minConfidence {
		slog.Warn("CodeAgent: confidence below threshold, skipping", "confidence", resp.Confidence, "threshold", a.minConfidence)
		return Plan{}, fmt.Errorf("low confidence: %.2f", resp.Confidence)
	}

	// Build the plan from the LLM response
	plan, err := a.buildPlan(task, resp)
	if err != nil {
		return Plan{}, err
	}

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

	// Inject the target source file content and semantic context.
	if ctx.SourceContent != "" {
		context.WriteString("\n--- Target source ---\n")
		context.WriteString(ctx.SourceContent)
		context.WriteString("\n--- End source ---\n")
	}

	return CodePrompt{
		Task:        task.Goal,
		FilePath:    task.Target,
		Language:    detectLanguage(task.Target),
		Context:     context.String(),
		Constraints: constraints.String(),
	}
}

// buildPlan converts an LLM response into an executable Plan.
//
// Returns (Plan, error) rather than only (Plan): validating the agent
// target through safeDirForFile may legitimately reject the input and
// the caller (Solve) MUST surface that as an error to the LLM, instead
// of silently producing a half-built plan.
func (a *CodeAgent) buildPlan(task Task, resp *CodeResponse) (Plan, error) {
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

	// Add a test run after the fix.
	//
	// SECURITY: task.Target flows in from the agent (LLM output) and could
	// contain shell metacharacters if the LLM is getting creative. We
	// validate it strictly before splicing it into any command line, and
	// we avoid "bash -c" entirely. The sandbox executor runs the
	// Command tokens via exec.Command(cmdStr, args...) so passing argv
	// directly means no shell interpretation happens — there is nothing
	// for ";rm -rf /" to attach to.
	//
	// We no longer cd into the per-file directory: the test invocation
	// runs go test ./... at the sandbox WorkDir root, which covers all
	// packages and is sufficient for a post-edit sanity check. If we
	// later need per-file test names, command Dir (argv-level cd, no
	// shell) can be threaded through Action in a separate, focused
	// change.
	if _, err := safeDirForFile(task.Target); err != nil {
		return Plan{}, fmt.Errorf("buildPlan: invalid target %q: %w", task.Target, err)
	}
	steps = append(steps, Action{
		Kind:    ActionRunCommand,
		Target:  "go test -run " + testNameForFile(task.Target) + " ./...",
		Command: []string{"go", "test", "-run", testNameForFile(task.Target), "./..."},
	})

	return Plan{Steps: steps}, nil
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

// DiffState represents the current state of diff patch parsing.
type DiffState int

const (
	// StateTextHunk: outside diff sections, copy as-is
	StateTextHunk DiffState = iota
	// StateDiffHeader: inside a diff header (---, +++, diff, index)
	StateDiffHeader
	// StateHunkMeta: hunk metadata line (@@)
	StateHunkMeta
	// StateHunkBody: hunk body (context, additions, deletions)
	StateHunkBody
)

// diffHeaderPrefix returns true if the line is a diff header line.
func diffHeaderPrefix(line string) bool {
	return strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") ||
		strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ")
}

// isHunkMeta returns true if the line is a hunk metadata line.
func isHunkMeta(line string) bool {
	return strings.HasPrefix(line, "@@") && strings.Contains(line, "@@")
}

// isDiffMetaLine returns true for diff metadata lines that should be skipped
// (new file mode, deleted file mode, Binary files, similarity index, etc).
func isDiffMetaLine(line string) bool {
	return strings.HasPrefix(line, "new file mode ") ||
		strings.HasPrefix(line, "deleted file mode ") ||
		strings.HasPrefix(line, "old mode ") ||
		strings.HasPrefix(line, "new mode ") ||
		strings.HasPrefix(line, "similarity index ") ||
		strings.HasPrefix(line, "rename from ") ||
		strings.HasPrefix(line, "rename to ") ||
		strings.HasPrefix(line, "copy from ") ||
		strings.HasPrefix(line, "copy to ") ||
		strings.HasPrefix(line, "Binary files ")
}

// stripDiffMarkers removes diff headers from a patch to get clean code.
// Implementation based on a state machine for maximum robustness.
// Rules:
//   - Diff headers (---, +++, diff, index): removed
//   - Hunk metadata (@@ ... @@): removed
//   - Extended metadata (new file mode, Binary files, etc.): removed
//   - Deletion lines (-) in a hunk: removed
//   - Context lines (space prefix) and addition lines (+): kept WITH their prefix
//   - Outside diff sections: copied as-is
//   - Empty lines in a hunk: kept
//   - Lines without standard prefix in a hunk: end of hunk, return to text
func stripDiffMarkers(patch string) string {
	lines := strings.Split(patch, "\n")
	var result []string
	state := StateTextHunk

	for _, line := range lines {
		switch state {
		case StateTextHunk:
			if diffHeaderPrefix(line) || isDiffMetaLine(line) {
				state = StateDiffHeader
				continue
			}
			// Start of hunk without header (e.g., minimal patch)
			if isHunkMeta(line) {
				state = StateHunkMeta
				continue
			}
			result = append(result, line)

		case StateDiffHeader:
			// @@ → start of hunk
			if isHunkMeta(line) {
				state = StateHunkMeta
				continue
			}
			// No more headers or metadata
			if diffHeaderPrefix(line) || isDiffMetaLine(line) {
				continue
			}
			// Normal line that is neither header nor hunk → return to text
			state = StateTextHunk
			result = append(result, line)

		case StateHunkMeta:
			// After @@, we enter the hunk body. The next line is evaluated
			// as part of this body.
			state = StateHunkBody
			fallthrough

		case StateHunkBody:
			// New file in the diff
			if diffHeaderPrefix(line) || isDiffMetaLine(line) {
				state = StateDiffHeader
				continue
			}
			// New hunk in the same file
			if isHunkMeta(line) {
				state = StateHunkMeta
				continue
			}
			// Deletion line
			if strings.HasPrefix(line, "-") {
				continue
			}
			// Context line (space prefix): keep the space
			if len(line) > 0 && line[0] == ' ' {
				result = append(result, line)
				continue
			}
			// Addition line (+ prefix): keep the +
			if len(line) > 0 && line[0] == '+' {
				result = append(result, line)
				continue
			}
			// Lines without standard prefix in the body: considered
			// as exiting the hunk (e.g., "diff --combined" produces
			// raw content after the hunk).
			if line == "" {
				result = append(result, line)
				continue
			}
			// Line with unrecognized prefix → end of hunk
			state = StateTextHunk
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// isDiff checks if the patch looks like a diff.
func isDiff(patch string) bool {
	return strings.HasPrefix(patch, "---") ||
		strings.HasPrefix(patch, "diff ") ||
		strings.HasPrefix(patch, "+++") ||
		strings.Contains(patch, "\n--- ") ||
		strings.Contains(patch, "\ndiff ") ||
		strings.Contains(patch, "\n+++ ")
}

// dirForFile returns the directory containing a file.
//
// Deprecated: dirForFile does not validate the path. Use safeDirForFile
// when the input could come from an untrusted source (LLM output, HTTP
// body, etc.).
func dirForFile(filePath string) string {
	idx := strings.LastIndex(filePath, "/")
	if idx < 0 {
		return "."
	}
	return filePath[:idx]
}

// safeDirForFile returns the clean directory containing filePath. It
// rejects any input that could drive a shell injection when later
// concatenated into a command line:
//
//   - empty string
//   - leading "-" (would be interpreted as a flag)
//   - any byte that is not [a-zA-Z0-9._/-] (no shell metacharacters,
//     no spaces, no quotes, no backticks, no $(), no ; no &&, no |, …)
//   - any ".." segment after filepath.Clean (catches "../../etc/passwd")
//   - absolute paths (defense in depth; project-relative paths only)
//
// The point is to give buildPlan — and any future caller that splices
// a path into argv-level execution — a single chokepoint that the test
// suite can pin. The regex is intentionally strict; if a legitimate
// path is rejected, prefer relaxing here rather than reintroducing
// concatenation risks at the call site.
func safeDirForFile(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(filePath, "-") {
		return "", fmt.Errorf("path starts with %q (flag-like)", filePath[:1])
	}
	for _, r := range filePath {
		if !(r == '/' || r == '.' || r == '_' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')) {
			return "", fmt.Errorf("path contains disallowed character %q", r)
		}
	}
	cleaned := filepath.Clean(filePath)
	if cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path %q escapes project root after Clean", filePath)
	}
	idx := strings.LastIndex(cleaned, "/")
	if idx < 0 {
		return ".", nil
	}
	return cleaned[:idx], nil
}

// testNameForFile generates a test name pattern for a source file.
func testNameForFile(filePath string) string {
	// e.g., add.go -> TestAdd
	base := strings.TrimSuffix(filePath, ".go")
	base = strings.TrimSuffix(base, ".py")
	// Capitalize first rune (replacement for the deprecated strings.Title since Go 1.18)
	if len(base) > 0 {
		runes := []rune(base)
		runes[0] = unicode.ToUpper(runes[0])
		return "Test" + string(runes)
	}
	return "Test" + base
}
