package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode"
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

// DiffState représente l'état courant du parsing d'un diff.
type DiffState int

const (
	// StateTextHunk: hors section diff, on copie tel quel
	StateTextHunk DiffState = iota
	// StateDiffHeader: dans l'en-tête d'un diff (---, +++, diff, index)
	StateDiffHeader
	// StateHunkMeta: ligne de métadonnées de hunk (@@)
	StateHunkMeta
	// StateHunkBody: corps du hunk (contexte, additions, suppressions)
	StateHunkBody
)

// diffHeaderPrefix retourne true si la ligne est un en-tête de diff.
func diffHeaderPrefix(line string) bool {
	return strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") ||
		strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ")
}

// isHunkMeta retourne true si la ligne est une métadonnée de hunk.
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
// Implémentation basée sur une machine à états pour une robustesse maximale.
// Règles :
//   - En-têtes de diff (---, +++, diff, index) : supprimés
//   - Métadonnées de hunk (@@ ... @@) : supprimées
//   - Métadonnées étendues (new file mode, Binary files, etc.) : supprimées
//   - Lignes de suppression (-) dans un hunk : supprimées
//   - Lignes de contexte (espace) et d'addition (+) : conservées AVEC leur préfixe
//   - Hors section diff : copié tel quel
//   - Lignes vides dans un hunk : conservées
//   - Lignes sans préfixe dans un hunk : fin du hunk, retour au texte
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
			// Début de hunk sans en-tête (ex: patch minimal)
			if isHunkMeta(line) {
				state = StateHunkMeta
				continue
			}
			result = append(result, line)

		case StateDiffHeader:
			// @@ → début du hunk
			if isHunkMeta(line) {
				state = StateHunkMeta
				continue
			}
			// Plus d'en-têtes ou de métadonnées
			if diffHeaderPrefix(line) || isDiffMetaLine(line) {
				continue
			}
			// Ligne normale qui n'est ni un en-tête ni un hunk → retour au texte
			state = StateTextHunk
			result = append(result, line)

		case StateHunkMeta:
			// Après @@, on entre dans le corps du hunk. On évalue la ligne
			// suivante comme faisant partie de ce corps.
			state = StateHunkBody
			fallthrough

		case StateHunkBody:
			// Nouveau fichier dans le diff
			if diffHeaderPrefix(line) || isDiffMetaLine(line) {
				state = StateDiffHeader
				continue
			}
			// Nouveau hunk dans le même fichier
			if isHunkMeta(line) {
				state = StateHunkMeta
				continue
			}
			// Ligne de suppression
			if strings.HasPrefix(line, "-") {
				continue
			}
			// Ligne de contexte (préfixe espace) : conserver l'espace
			if len(line) > 0 && line[0] == ' ' {
				result = append(result, line)
				continue
			}
			// Ligne d'addition (préfixe +) : conserver le +
			if len(line) > 0 && line[0] == '+' {
				result = append(result, line)
				continue
			}
			// Lignes sans préfixe standard dans le corps : considéré
			// comme une sortie du hunk (ex: "diff --combined" produit
			// du contenu brut après le hunk).
			if line == "" {
				result = append(result, line)
				continue
			}
			// Ligne sans préfixe reconnu → fin du hunk
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
	// Capitalize first rune (replacement de strings.Title déprécié depuis Go 1.18)
	if len(base) > 0 {
		runes := []rune(base)
		runes[0] = unicode.ToUpper(runes[0])
		return "Test" + string(runes)
	}
	return "Test" + base
}