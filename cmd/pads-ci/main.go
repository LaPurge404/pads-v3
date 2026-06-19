package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type SystemState struct {
	Mode           string    `json:"Mode"`
	Sequence       int       `json:"Sequence"`
	DetectorWindow []float64 `json:"DetectorWindow"`
	StabilityScore float64   `json:"StabilityScore"`
}

func main() {
	apiURL := flag.String("url", "http://127.0.0.1:8080", "URL de l'API PADS")
	token := flag.String("token", "", "Token d'authentification")
	flag.Parse()

	if *token == "" {
		fmt.Fprintln(os.Stderr, "Erreur : token obligatoire (-token)")
		os.Exit(1)
	}

	// 1. Candidate score = ratio of passed tests (normalized 0-100)
	candidateScore := getTestScore()
	slog.Info("score candidat calculé", "score", candidateScore)

	// 2. Current score = StabilityScore of the system state
	currentScore, err := getCurrentStability(*apiURL, *token)
	if err != nil {
		slog.Warn("impossible de récupérer l'état, utilisation de 0", "error", err)
		currentScore = 0
	}
	slog.Info("score de stabilité courant", "score", currentScore)

	// 3. Submit the evolution
	resp, err := postEvolve(*apiURL, *token, int(candidateScore), int(currentScore), 1.0, "stable")
	if err != nil {
		slog.Error("échec de la soumission", "error", err)
		os.Exit(1)
	}
	slog.Info("évolution soumise", "id", resp["id"])

	// 4. Wait for processing and retrieve the new stability score
	newScore, err := waitForNewStability(*apiURL, *token)
	if err != nil {
		slog.Error("timeout attente worker", "error", err)
		os.Exit(1)
	}
	slog.Info("nouveau score de stabilité", "score", newScore)

	// 5. Decision
	if newScore > currentScore {
		fmt.Println("✅ Stabilité améliorée, commit accepté.")
		os.Exit(0)
	}
	fmt.Println("❌ Dégradation ou stabilité insuffisante, commit rejeté.")
	os.Exit(1)
}

// getTestScore runs go test and computes the ratio (passed / total) * 100
func getTestScore() float64 {
	cmd := exec.Command("go", "test", "./...", "-count=1")
	output, _ := cmd.CombinedOutput()
	outStr := string(output)
	lines := strings.Split(outStr, "\n")
	passed := 0
	failed := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ok ") {
			passed++
		} else if strings.HasPrefix(trimmed, "FAIL ") {
			failed++
		}
	}
	total := passed + failed
	if total == 0 {
		return 0
	}
	return (float64(passed) / float64(total)) * 100.0
}

func getCurrentStability(apiURL, token string) (float64, error) {
	state, err := fetchState(apiURL, token)
	if err != nil {
		return 0, err
	}
	return state.StabilityScore, nil
}

func fetchState(apiURL, token string) (*SystemState, error) {
	req, _ := http.NewRequest("GET", apiURL+"/state", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("état inaccessible (%d)", resp.StatusCode)
	}
	var state SystemState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func postEvolve(apiURL, token string, candidate, current int, weight float64, mode string) (map[string]string, error) {
	body := map[string]interface{}{
		"candidate": candidate,
		"current":   current,
		"weight":    weight,
		"mode":      mode,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", apiURL+"/evolve", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("échec requête (%d)", resp.StatusCode)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("décodage réponse CI: %w", err)
	}
	return result, nil
}

func waitForNewStability(apiURL, token string) (float64, error) {
	timeout := time.After(15 * time.Second)
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			return 0, fmt.Errorf("timeout")
		case <-tick.C:
			state, err := fetchState(apiURL, token)
			if err != nil {
				continue
			}
			if state.Sequence > 0 {
				return state.StabilityScore, nil
			}
		}
	}
}
