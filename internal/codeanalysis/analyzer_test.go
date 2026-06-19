package codeanalysis

import (
	"strings"
	"testing"
)

// TestResultSummary verifies summary formatting.
func TestResultSummary(t *testing.T) {
	r := Result{
		TestsPassed:  38,
		TestsFailed:  2,
		TestsAllPass: false,
		LintIssues:   12,
		Coverage:     49.4,
		Score:        71.2,
	}
	summary := r.Summary()
	if !strings.Contains(summary, "38") {
		t.Errorf("summary should contain TestsPassed: %s", summary)
	}
	if !strings.Contains(summary, "12") {
		t.Errorf("summary should contain LintIssues: %s", summary)
	}
	if !strings.Contains(summary, "49.4") {
		t.Errorf("summary should contain Coverage: %s", summary)
	}
}

// TestExtractCoverage verifies parsing of go test -cover output.
// Does NOT cover the "total: ... XX%" format (go tool cover -func) because
// analyze.go uses "go test -cover" which formats differently.
func TestExtractCoverage(t *testing.T) {
	tests := []struct {
		output string
		want   float64
	}{
		{"ok   pads-v3/internal/agent\tcoverage: 18.9% of statements", 18.9},
		{"ok   pads-v3/internal/policy\tevasion\tcoverage: 78.5% of statements", 78.5},
		{"?    pads-v3/cmd/evolution-api\tcoverage: 0.0% of statements", 0.0},
		{"no coverage output here", 0.0},
		{"", 0.0},
		{"coverage: 100%", 100.0},
		{"coverage: 0% of statements", 0.0},
	}
	for _, tt := range tests {
		got := extractCoverage(tt.output)
		if got != tt.want {
			t.Errorf("extractCoverage(%q) = %.2f, want %.2f",
				tt.output[:min(60, len(tt.output))], got, tt.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
