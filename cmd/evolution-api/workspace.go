package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// workspaceCacheTTL bounds how often `go test ./...` runs for the `/workspace`
// endpoint. Without this guard, an attacker calling `/workspace` 10x/min could
// chain long `go test` invocations and saturate the server. 10s is short
// enough that real "is branch green?" dashboards stay fresh, and long enough
// to absorb accidental polling loops.
const workspaceCacheTTL = 10 * time.Second

// workspaceCache holds the latest (passed, failed) counts plus the timestamp
// at which they were captured. Both fields are guarded by mu.
type workspaceCache struct {
	mu       sync.Mutex
	cached   bool
	at       time.Time
	passed   int
	failed   int
}

var wsCache workspaceCache

// workspaceHandler returns the Git repository state and test results.
//
// The handler responds immediately with the cached result if the cache is
// fresh. Cache misses (or first call) trigger a single `go test ./...` run
// synchronously; future calls within workspaceCacheTTL are served from cache.
// This converts the DoS vector (10 req/min × N users × long `go test`) into
// at most one `go test` per workspaceCacheTTL window.
func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	passed, failed, _, cached := runTestsCached()
	resp := map[string]interface{}{
		"gitBranch":  getGitBranch(),
		"gitStatus":  getGitStatus(),
		"testPassed": passed,
		"testFailed": failed,
		"cached":     cached,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// runTestsCached returns the current `(passed, failed)` counts from cache if
// fresh, otherwise runs them and stores the result. The third return value is
// the latency of the underlying test run (zero on cache hit). The fourth is
// `true` when the result was served from cache.
func runTestsCached() (passed, failed int, took time.Duration, cached bool) {
	wsCache.mu.Lock()
	if wsCache.cached && time.Since(wsCache.at) < workspaceCacheTTL {
		p, f := wsCache.passed, wsCache.failed
		wsCache.mu.Unlock()
		return p, f, 0, true
	}
	wsCache.mu.Unlock()

	start := time.Now()
	p, f := runTestsOnce()
	took = time.Since(start)

	wsCache.mu.Lock()
	wsCache.passed = p
	wsCache.failed = f
	wsCache.at = time.Now()
	wsCache.cached = true
	wsCache.mu.Unlock()

	return p, f, took, false
}

// runTestsOnce executes "go test ./..." once and returns (passed, failed) counts.
func runTestsOnce() (passed, failed int) {
	cmd := exec.Command("go", "test", "./...", "-count=1")
	out, err := cmd.CombinedOutput()
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "ok  ") {
			passed++
		} else if strings.HasPrefix(l, "FAIL ") {
			failed++
		}
	}
	// If the command failed globally but no FAIL lines detected, count 1 failure.
	if err != nil && failed == 0 {
		failed = 1
	}
	return passed, failed
}

// getGitBranch returns the current Git branch or "not available".
func getGitBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "non disponible"
	}
	return strings.TrimSpace(string(out))
}

// getGitStatus returns the Git status or "clean" if nothing to report.
func getGitStatus() string {
	cmd := exec.Command("git", "status", "--short")
	out, err := cmd.Output()
	if err != nil {
		return "non disponible"
	}
	if len(out) == 0 {
		return "(propre)"
	}
	return strings.TrimSpace(string(out))
}
