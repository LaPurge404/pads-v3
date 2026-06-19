package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeFile(t *testing.T) {
	// Create a temp Go file to analyze
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testpkg", "example.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `package testpkg

// ExportedFunc is a documented function.
func ExportedFunc(a int, b string) error {
	return InternalHelper(a)
}

// internalFunc is not exported.
func internalFunc(x int) int {
	return x * 2
}

// MyType is an exported type.
type MyType struct {
	Field1 int
	Field2 string
}

const MyConst = 42

var myVar = "hello"
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	a := NewAnalyzer(tmpDir)
	sum, err := a.AnalyzeFile(testFile)
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}

	if sum.FilePath != testFile {
		t.Errorf("FilePath: got %q, want %q", sum.FilePath, testFile)
	}

	if sum.ExportedCount < 2 {
		t.Errorf("ExportedCount: got %d, want >= 2 (ExportedFunc, MyType, MyConst)", sum.ExportedCount)
	}

	if len(sum.Symbols) == 0 {
		t.Error("no symbols extracted")
	}

	// Check ExportedFunc is found and is exported
	var foundExportedFunc bool
	for _, s := range sum.Symbols {
		if s.Name == "ExportedFunc" {
			foundExportedFunc = true
			if !s.Exported {
				t.Error("ExportedFunc should be exported")
			}
			if s.Kind != KindFunc {
				t.Errorf("ExportedFunc Kind: got %v, want KindFunc", s.Kind)
			}
		}
	}
	if !foundExportedFunc {
		t.Error("ExportedFunc not found in symbols")
	}
}

func TestAnalyzeDiff(t *testing.T) {
	before := `package pkg

func Foo() int {
	return 42
}

type Bar struct{}
`
	after := `package pkg

// Foo has been updated.
func Foo(a int) string {
	return "modified"
}

type Bar struct {
	NewField int
}

type NewType struct{}
`

	a := NewAnalyzer("/fake")
	sum, err := a.AnalyzeDiff("pkg/example.go", before, after)
	if err != nil {
		t.Fatalf("AnalyzeDiff: %v", err)
	}

	if sum.ModifiedCount == 0 {
		t.Error("AnalyzeDiff: expected at least 1 modified symbol")
	}

	if sum.ModTypeCounts[ModificationSignature] == 0 {
		t.Error("AnalyzeDiff: expected at least 1 signature modification")
	}
}

func TestRiskScore(t *testing.T) {
	cases := []struct {
		name        string
		symbols     []Symbol
		wantMinRisk float64
		wantMaxRisk float64
	}{
		{
			name:        "no symbols",
			symbols:     []Symbol{},
			wantMinRisk: 0.0,
			wantMaxRisk: 0.0,
		},
		{
			name: "internal body change only",
			symbols: []Symbol{
				{Name: "internal", Exported: false, ModType: ModificationBody},
			},
			wantMinRisk: 0.0,
			wantMaxRisk: 0.2,
		},
		{
			name: "exported signature change",
			symbols: []Symbol{
				{Name: "Exported", Exported: true, ModType: ModificationSignature},
			},
			wantMinRisk: 0.2,
			wantMaxRisk: 1.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sum := &Summary{Symbols: tc.symbols, ExportedCount: 0, ModTypeCounts: make(map[ModificationType]int)}
			for _, s := range tc.symbols {
				if s.Exported {
					sum.ExportedCount++
				}
				sum.ModTypeCounts[s.ModType]++
			}
			a := NewAnalyzer("/fake")
			a.computeRisk(sum)
			if sum.RiskScore < tc.wantMinRisk || sum.RiskScore > tc.wantMaxRisk {
				t.Errorf("RiskScore: got %.2f, want [%.2f, %.2f]", sum.RiskScore, tc.wantMinRisk, tc.wantMaxRisk)
			}
		})
	}
}

func TestCallGraph(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "callgraph", "main.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `package main

func helper() int {
	return 1
}

func middle() int {
	return helper()
}

func main() {
	helper()
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	a := NewAnalyzer(tmpDir)
	sum, err := a.AnalyzeFile(testFile)
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}

	// helper should be called by both middle and main
	var helperSym Symbol
	for _, s := range sum.Symbols {
		if s.Name == "helper" {
			helperSym = s
			break
		}
	}
	if len(helperSym.Callers) < 2 {
		t.Errorf("helper callers: got %v, want at least [middle, main]", helperSym.Callers)
	}

	// middle should call helper
	var middleSym Symbol
	for _, s := range sum.Symbols {
		if s.Name == "middle" {
			middleSym = s
			break
		}
	}
	if len(middleSym.Callees) == 0 {
		t.Error("middle should call helper")
	}
}

func TestFormatModType(t *testing.T) {
	tests := []struct {
		mt   ModificationType
		want string
	}{
		{ModificationSignature, "signature"},
		{ModificationBody, "body"},
		{ModificationComment, "comment"},
		{ModificationImport, "import"},
		{ModificationExport, "export"},
		{ModificationUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := FormatModType(tt.mt); got != tt.want {
			t.Errorf("FormatModType(%v): got %q, want %q", tt.mt, got, tt.want)
		}
	}
}

func TestFormatKind(t *testing.T) {
	tests := []struct {
		k    SymbolKind
		want string
	}{
		{KindFunc, "function"},
		{KindType, "type"},
		{KindVar, "variable"},
		{KindConst, "constant"},
		{KindMethod, "method"},
		{KindUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := FormatKind(tt.k); got != tt.want {
			t.Errorf("FormatKind(%v): got %q, want %q", tt.k, got, tt.want)
		}
	}
}

func TestAnalyzeFile_InvalidPath(t *testing.T) {
	a := NewAnalyzer("/nonexistent")
	_, err := a.AnalyzeFile("/nonexistent/file.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFormatSummary(t *testing.T) {
	sum := &Summary{
		FilePath:      "test.go",
		ModifiedCount: 2,
		ExportedCount: 1,
		RiskScore:     0.5,
		RiskReasons:   []string{"exported symbol changed"},
		Symbols: []Symbol{
			{Name: "Foo", Kind: KindFunc, Exported: true, ModType: ModificationSignature},
		},
	}
	output := FormatSummary(sum)
	if !strings.Contains(output, "test.go") {
		t.Error("FormatSummary should contain file path")
	}
	if !strings.Contains(output, "0.50") {
		t.Error("FormatSummary should contain risk score")
	}
}
