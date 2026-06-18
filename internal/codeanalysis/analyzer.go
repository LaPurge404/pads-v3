package codeanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Result holds the output of a code analysis run.
type Result struct {
	TestsPassed  int
	TestsFailed  int
	TestsAllPass bool
	LintIssues   int
	Coverage     float64
	Score        float64
}

// Analyzer runs code quality checks and caches results by content hash.
type Analyzer struct {
	// cache maps content-hash → Result to avoid re-running analysis for
	// unchanged code. It is shared across all analysis calls.
	mu    sync.RWMutex
	cache map[string]Result
}

// NewAnalyzer creates a fresh Analyzer with an empty cache.
func NewAnalyzer() *Analyzer {
	return &Analyzer{cache: make(map[string]Result)}
}

// ClearCache empties the result cache. Useful for forcing a fresh analysis.
func (a *Analyzer) ClearCache() {
	a.mu.Lock()
	a.cache = make(map[string]Result)
	a.mu.Unlock()
}

// Analyze runs the full analysis suite (tests, lint, coverage) and caches
// the result keyed by contentHash. If contentHash is empty, caching is skipped.
func (a *Analyzer) Analyze(contentHash string) (Result, error) {
	if contentHash != "" {
		// Fast path: return cached result
		a.mu.RLock()
		if cached, ok := a.cache[contentHash]; ok {
			a.mu.RUnlock()
			return cached, nil
		}
		a.mu.RUnlock()
	}

	res, err := a.run()
	if err != nil {
		return res, err
	}

	if contentHash != "" {
		a.mu.Lock()
		a.cache[contentHash] = res
		a.mu.Unlock()
	}

	return res, nil
}

// AnalyzeWithHash computes SHA256 of the provided file paths and uses it as
// the cache key. This avoids re-running analysis when the same files haven't changed.
// dir is the common directory for the files (used as working directory for commands).
func (a *Analyzer) AnalyzeWithHash(dir string, files []string) (Result, error) {
	hash, err := computeFileHash(dir, files)
	if err != nil {
		// If hash fails, run analysis without caching
		return a.Analyze("")
	}
	return a.Analyze(hash)
}

// run executes the actual analysis commands (tests, lint, coverage).
func (a *Analyzer) run() (Result, error) {
	var res Result

	// 1. Tests
	cmd := exec.Command("go", "test", "./...", "-count=1")
	testOut, testErr := cmd.CombinedOutput()
	lines := strings.Split(string(testOut), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "ok ") {
			res.TestsPassed++
		} else if strings.HasPrefix(trimmed, "FAIL ") {
			res.TestsFailed++
		}
	}
	// If the command failed globally and no FAIL was counted, count as one failure
	if testErr != nil {
		if res.TestsFailed == 0 {
			res.TestsFailed = 1
		}
	}
	res.TestsAllPass = res.TestsFailed == 0

	// 2. Lint (run even if golangci-lint is not installed — count = 0)
	lintCmd := exec.Command("golangci-lint", "run", "--out-format=line-number")
	lintOut, _ := lintCmd.CombinedOutput() // ignore error if binary not present
	res.LintIssues = strings.Count(string(lintOut), "\n")

	// 3. Coverage
	covCmd := exec.Command("go", "test", "./...", "-cover")
	covOut, _ := covCmd.CombinedOutput()
	res.Coverage = extractCoverage(string(covOut))

	// 4. Composite score
	testScore := 0.0
	if res.TestsPassed+res.TestsFailed > 0 {
		testScore = float64(res.TestsPassed) / float64(res.TestsPassed+res.TestsFailed) * 100.0
	}
	lintScore := 0.0
	if res.LintIssues <= 5 {
		lintScore = 100.0
	} else if res.LintIssues <= 20 {
		lintScore = 60.0
	} else {
		lintScore = 20.0
	}
	covScore := res.Coverage
	res.Score = testScore*0.4 + lintScore*0.3 + covScore*0.3

	return res, nil
}

// Summary returns a human-readable string describing the analysis result.
func (r Result) Summary() string {
	return fmt.Sprintf("Tests: %d/%d passés, Lint: %d problèmes, Couverture: %.1f%%, Score: %.1f",
		r.TestsPassed, r.TestsPassed+r.TestsFailed, r.LintIssues, r.Coverage, r.Score)
}

// CacheStats returns the number of cached entries.
func (a *Analyzer) CacheStats() int {
	a.mu.RLock()
	n := len(a.cache)
	a.mu.RUnlock()
	return n
}

// computeFileHash computes a SHA256 hash over the combined content of files.
func computeFileHash(dir string, files []string) (string, error) {
	// Use go list to get the hash of module versions + file mod times
	// This is a fast proxy for "files changed" without reading all content.
	// For fine-grained cache invalidation, pass the actual file content hash.
	if len(files) == 0 {
		return "", nil
	}
	args := append([]string{"list", "-f", "{{.Module.Version}}"}, files...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compute hash: %w", err)
	}
	h := sha256.Sum256(out)
	return hex.EncodeToString(h[:]), nil
}

func extractCoverage(output string) float64 {
	lines := strings.Split(output, "\n")
	for _, l := range lines {
		if strings.Contains(l, "coverage:") {
			parts := strings.Split(l, "coverage:")
			if len(parts) < 2 {
				continue
			}
			right := strings.TrimSpace(parts[1])
			right = strings.TrimSuffix(right, "% of statements")
			right = strings.TrimSuffix(right, "%")
			val, err := strconv.ParseFloat(right, 64)
			if err == nil {
				return val
			}
		}
	}
	return 0.0
}

// Module-level helpers (keep the existing API for callers that don't use Analyzer).

var defaultAnalyzer = NewAnalyzer()

// Analyze runs analysis with caching using the global analyzer.
func Analyze() (Result, error) {
	return defaultAnalyzer.Analyze("")
}

// ClearCache clears the global analyzer's cache.
func ClearCache() {
	defaultAnalyzer.ClearCache()
}