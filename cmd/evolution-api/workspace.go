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

	resp := map[string]interface{}{
		"gitBranch":  getGitBranch(),
		"gitStatus":  getGitStatus(),
		"testPassed": getTestPassedCount(),
		"testFailed": getTestFailedCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

// getTestPassedCount runs go test and counts the packages that passed.
func getTestPassedCount() int {
	cmd := exec.Command("go", "test", "./...", "-count=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// the command failed, but we still count the ok packages for the dashboard
	}
	lines := strings.Split(string(out), "\n")
	count := 0
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "ok ") {
			count++
		}
	}
	return count
}

// getTestFailedCount counts the packages that failed.
func getTestFailedCount() int {
	cmd := exec.Command("go", "test", "./...", "-count=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// same
	}
	lines := strings.Split(string(out), "\n")
	count := 0
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "FAIL ") {
			count++
		}
	}
	// If the command failed globally but no FAIL detected, we count 1 failure
	if err != nil && count == 0 {
		count = 1
	}
	return count
}
