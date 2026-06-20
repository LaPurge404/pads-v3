package main

import (
	"testing"
)

// TestParseGoTestOutput verifies the scoring logic with various go test outputs.
func TestParseGoTestOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   float64
	}{
		{
			name:   "all ok",
			output: "ok  \tpads-v3/internal/agent\t8.392s\nok  \tpads-v3/internal/autonomous\t0.543s\n",
			want:   100.0,
		},
		{
			name:   "all fail",
			output: "FAIL\tpads-v3/internal/agent\nFAIL\tpads-v3/internal/autonomous\n",
			want:   0.0,
		},
		{
			name:   "partial failure",
			output: "ok  \tpads-v3/internal/agent\t8.392s\nFAIL\tpads-v3/internal/autonomous\n",
			want:   50.0,
		},
		{
			name:   "no tests found",
			output: "",
			want:   0.0,
		},
		{
			name:   "no ok or fail lines",
			output: "go test ./...\n",
			want:   0.0,
		},
		{
			name:   "only ok lines",
			output: "ok  \tpads-v3/pkg-a\nok  \tpads-v3/pkg-b\nok  \tpads-v3/pkg-c\n",
			want:   100.0,
		},
		{
			name:   "mixed with extra whitespace",
			output: "  ok  \tpads-v3/pkg-a\n FAIL\tpads-v3/pkg-b",
			want:   50.0,
		},
		{
			name:   "three passing one failing",
			output: "ok   \tpkg-a\nok   \tpkg-b\nFAIL\tpkg-c\nok   \tpkg-d\n",
			want:   75.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGoTestOutput(tt.output)
			if got != tt.want {
				t.Errorf("parseGoTestOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}