package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

// workspaceHandler renvoie l'état du dépôt Git et des tests.
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

// getGitBranch retourne la branche Git actuelle ou "non disponible".
func getGitBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "non disponible"
	}
	return strings.TrimSpace(string(out))
}

// getGitStatus retourne le statut Git ou "propre" si rien à signaler.
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

// getTestPassedCount exécute go test et compte les packages réussis.
func getTestPassedCount() int {
	cmd := exec.Command("go", "test", "./...", "-count=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// la commande a échoué, on compte quand même les ok pour le tableau de bord
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

// getTestFailedCount compte les packages qui ont échoué.
func getTestFailedCount() int {
	cmd := exec.Command("go", "test", "./...", "-count=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// idem
	}
	lines := strings.Split(string(out), "\n")
	count := 0
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "FAIL ") {
			count++
		}
	}
	// Si la commande a échoué globalement mais aucun FAIL détecté, on compte 1 échec
	if err != nil && count == 0 {
		count = 1
	}
	return count
}
