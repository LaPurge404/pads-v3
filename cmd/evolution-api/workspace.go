package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

// workspaceHandler returns the Git repository state and test results.
func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	passed, failed := runTestsOnce()

	resp := map[string]interface{}{
		"gitBranch":  getGitBranch(),
		"gitStatus":  getGitStatus(),
		"testPassed": passed,
		"testFailed": failed,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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