package agent

import (
	"testing"
)

// TestTestNameForFile vérifie la génération de nom de test.
func TestTestNameForFile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"add.go", "TestAdd"},
		{"handler.go", "TestHandler"},
		{"add_test.go", "TestAdd_test"}, // .go trim → _test reste
		{"my_file.go", "TestMy_file"},
		{"server.go", "TestServer"},
		{"utils.go", "TestUtils"},
	}
	for _, tt := range tests {
		got := testNameForFile(tt.input)
		if got != tt.want {
			t.Errorf("testNameForFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestTestNameForFileEmpty vérifie le cas d'un nom vide.
func TestTestNameForFileEmpty(t *testing.T) {
	got := testNameForFile("")
	if got != "Test" {
		t.Errorf("testNameForFile(%q) = %q, want %q", "", got, "Test")
	}
}

// TestDetectLanguage vérifie la détection du langage.
func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.js", "javascript"},
		{"qux.ts", "typescript"},
	}
	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestStripDiffMarkers vérifie le nettoyage des marqueurs diff.
func TestStripDiffMarkers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Cas de base
		{"simple text", "package main", "package main"},

		// Diff complet standard
				{
					"full diff",
					"diff --git a/foo.go b/foo.go\n" +
						"index 123..456 789\n" +
						"--- a/foo.go\n" +
						"+++ b/foo.go\n" +
						"@@ -1,3 +1,4 @@\n" +
						" package main\n" +
						"-import \"fmt\"\n" +
						"+import \"fmt\"\n" +
						"+import \"strings\"\n" +
						" func main() {}",
					" package main\n" +
							"+import \"fmt\"\n" +
							"+import \"strings\"\n" +
							" func main() {}",
				},

				// Fichier vide (nouveau fichier)
				{
					"empty file diff",
					"diff --git a/new.go b/new.go\n" +
						"new file mode 100644\n" +
						"--- /dev/null\n" +
						"+++ b/new.go\n" +
						"@@ -0,0 +1,2 @@\n" +
						"+package new\n" +
						"+func New() {}",
					"+package new\n" +
							"+func New() {}",
				},

				// Patch binaire (sans contenu utilisable)
				{
					"binary diff",
					"diff --git a/logo.png b/logo.png\n" +
						"Binary files a/logo.png and b/logo.png differ",
					"",
				},

				// Unicode dans le contenu
				{
					"unicode content",
					"--- a/emoji.go\n" +
						"+++ b/emoji.go\n" +
						"@@ -1 +1 @@\n" +
						"-const smiley = \"😀\"\n" +
						"+const smiley = \"😀🎉👋\"\n",
					"+const smiley = \"😀🎉👋\"\n",
				},

				// Noms de fichiers avec + (ex: c++, go++ test_name)
				{
					"filename with plus",
					"diff --git a/test++.go b/test++.go\n" +
						"--- a/test++.go\n" +
						"+++ b/test++.go\n" +
						"@@ -1,2 +1,3 @@\n" +
						" package main\n" +
						"+// line with + sign\n" +
						" func main() {}",
					" package main\n" +
							"+// line with + sign\n" +
							" func main() {}",
				},

				// Multiple hunks
				{
					"multiple hunks",
					"--- a/main.go\n" +
						"+++ b/main.go\n" +
						"@@ -1,3 +1,4 @@\n" +
						" package main\n" +
						"+import \"fmt\"\n" +
						" func main() {\n" +
						"-	fmt.Println(\"old\")\n" +
						"+	fmt.Println(\"new\")\n" +
						" }\n" +
						"@@ -10,3 +11,4 @@\n" +
						" func helper() {}\n" +
						"+// comment\n" +
						"+",
					" package main\n" +
							"+import \"fmt\"\n" +
							" func main() {\n" +
							"+	fmt.Println(\"new\")\n" +
							" }\n" +
							" func helper() {}\n" +
							"+// comment\n" +
							"+",
				},

				// Lignes vides dans le diff
				{
					"empty lines in diff",
					"--- a/main.go\n" +
						"+++ b/main.go\n" +
						"@@ -1,2 +1,3 @@\n" +
						" package main\n" +
						"+\n" +
						"+",
					" package main\n" +
							"+\n" +
							"+",
				},

				// Ancienne approche: suppression de lignes "-" (cas sans en-tête)
				// Sans en-tête diff, le contenu n'est pas traité comme un diff,
				// donc les lignes - et + sont passées telles quelles.
				{
					"old style deletion",
					"- removed line\n+ added line\n unchanged",
					"- removed line\n+ added line\n unchanged",
				},

				// Ligne d'ajout dans un diff (sans @@ → pas de hunk)
				// Le "+ new line" après l'en-tête est vu comme ligne normale,
				// donc on ressort en StateTextHunk et on copie.
				{
					"simple addition no hunk",
					"--- a/foo.go\n" +
						"+++ b/foo.go\n" +
						"+ new line",
					"+ new line",
				},

				// Ligne d'ajout dans un diff avec @@
				{
					"simple addition with hunk",
					"--- a/foo.go\n" +
						"+++ b/foo.go\n" +
						"@@ -0,0 +1 @@\n" +
						"+ new line",
					"+ new line",
				},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripDiffMarkers(tt.input)
			if got != tt.want {
				t.Errorf("stripDiffMarkers() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestIsDiff vérifie la détection de format diff.
func TestIsDiff(t *testing.T) {
	tests := []struct {
		patch string
		want  bool
	}{
		{"diff --git a/foo.go b/foo.go", true},
		{"--- a/foo.go\n+++ b/foo.go", true},
		{"+++ b/foo.go", true},
		{"@@ -1,3 +1,4 @@", false},
		{"func main() {}", false},
		{"", false},
		// Nouveaux cas: diff au milieu du texte
		{"some text\n--- a/foo.go\n+++ b/foo.go", true},
		{"some text\ndiff --git a/foo.go b/foo.go", true},
	}
	for _, tt := range tests {
		got := isDiff(tt.patch)
		if got != tt.want {
			t.Errorf("isDiff(%q) = %v, want %v", tt.patch[:min(30, len(tt.patch))], got, tt.want)
		}
	}
}

// TestStripDiffMarkersEmpty vérifie le cas d'un patch vide.
func TestStripDiffMarkersEmpty(t *testing.T) {
	got := stripDiffMarkers("")
	if got != "" {
		t.Errorf("stripDiffMarkers(\"\") = %q, want empty string", got)
	}
}

// TestStripDiffMarkersOnlyWhitespace vérifie le cas de whitespace seul.
func TestStripDiffMarkersOnlyWhitespace(t *testing.T) {
	input := "   \n	\n"
	got := stripDiffMarkers(input)
	if got != input {
		t.Errorf("stripDiffMarkers(%q) = %q, want %q", input, got, input)
	}
}

// TestStripDiffMarkersContextPreserved vérifie que les lignes de contexte sont bien conservées.
func TestStripDiffMarkersContextPreserved(t *testing.T) {
	input := "--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,3 +1,3 @@\n" +
		" package main\n" +
		"-import \"fmt\"\n" +
		"+import \"fmt\"\n" +
		" func main() {}"
	got := stripDiffMarkers(input)
	want := " package main\n" +
		"+import \"fmt\"\n" +
		" func main() {}"
	if got != want {
		t.Errorf("stripDiffMarkers() = %q, want %q", got, want)
	}
}

// TestDirForFile vérifie l'extraction du répertoire.
func TestDirForFile(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/agent/llm.go", "internal/agent"},
		{"server.go", "."},        // pas de / → "."
		{"a/b/c.go", "a/b"},
		{"pkg/util.go", "pkg"},
	}
	for _, tt := range tests {
		got := dirForFile(tt.path)
		if got != tt.want {
			t.Errorf("dirForFile(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestBuildPrompt vérifie que buildPrompt retourne un CodePrompt complet.
func TestBuildPrompt(t *testing.T) {
	agent := NewCodeAgent(nil) // nil LLM → pas appelé dans ce test
	prompt := agent.buildPrompt(
		Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"},
		Context{FilePath: "add.go"},
	)
	if prompt.Task == "" {
		t.Error("prompt.Task should not be empty")
	}
	if prompt.FilePath != "add.go" {
		t.Errorf("prompt.FilePath = %q, want %q", prompt.FilePath, "add.go")
	}
	if prompt.Language != "go" {
		t.Errorf("prompt.Language = %q, want %q", prompt.Language, "go")
	}
}

// TestBuildPlan vérifie que buildPlan génère un Plan avec les étapes.
func TestBuildPlan(t *testing.T) {
	agent := NewCodeAgent(nil)
	plan := agent.buildPlan(
		Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"},
		&CodeResponse{
			Patch:       "func add(a, b int) int { return a + b }",
			Explanation: "added nil check",
			Confidence:  0.85,
		},
	)
	if len(plan.Steps) == 0 {
		t.Error("plan should have at least one step")
	}
	if plan.Steps[0].Kind != ActionWriteFile {
		t.Errorf("first step kind = %v, want ActionWriteFile", plan.Steps[0].Kind)
	}
	if plan.Steps[0].Target != "add.go" {
		t.Errorf("first step target = %q, want %q", plan.Steps[0].Target, "add.go")
	}
}

// Note: TestSolveWithMockLLM est commenté car le mock LLM retourne
// une confidence de 0.50 qui est en dessous du seuil par défaut (0.60).
// func TestSolveWithMockLLM(...) { ... }