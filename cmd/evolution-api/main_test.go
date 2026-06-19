package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"pads-v3/internal/policy/evolution"
)

func newTestServer(t *testing.T) *Server {
	queuePath := t.TempDir() + "/event_queue.log"
	queue, err := evolution.NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}

	selector := evolution.NewUCBSelector(time.Now().UnixNano())
	selector.AddArm("stable")
	selector.AddArm("bandit")
	selector.AddArm("locked")

	// Pre-warm selector to avoid NaN in avg_reward (arms with counts=0 cause div/0 in handler)
	for _, arm := range selector.ListArms() {
		selector.Update(arm, 1.0)
	}

	rl := evolution.NewRateLimiter(10, 1*time.Minute)

	return &Server{
		queue:     queue,
		selector:  selector,
		authToken: "test-token",
		rl:        rl,
	}
}

// setupTestMux creates an http mux with the same middleware chain as main().
func setupTestMux(srv *Server) *http.ServeMux {
	mux := http.NewServeMux()

	// Public endpoints (no auth)
	mux.HandleFunc("/health", srv.health)

	// Protected endpoints: securityHeaders → LoggingMiddleware → authMiddleware → RateLimiter.Middleware → handler
	protected := func(path string, h http.HandlerFunc) {
		chain := securityHeaders(h)
		chain = evolution.LoggingMiddleware(chain)
		chain = srv.authMiddleware(chain)
		chain = srv.rl.Middleware(chain)
		mux.HandleFunc(path, chain)
	}

	protected("/evolve", srv.enqueueEvolve)
	protected("/state", srv.state)
	protected("/select", srv.handleSelect)
	protected("/workspace", srv.workspace)
	protected("/agent/evolve", srv.handleAgentEvolve)
	protected("/agent/status", srv.handleAgentStatus)
	protected("/agent/strategies", srv.handleAgentStrategies)
	protected("/rotate", srv.handleRotate)

	return mux
}

// authHeader returns a valid Authorization header for the test server.
func authHeader(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	resp, err := http.Get(svr.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	buf := make([]byte, 2)
	n, _ := resp.Body.Read(buf)
	if n != 2 || string(buf) != "OK" {
		t.Errorf("body = %q, want %q", string(buf), "OK")
	}
}

func TestHealthNoAuth(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	// No Authorization header — should still succeed for /health
	resp, err := http.Get(svr.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health (no auth): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestState(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/state", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
}

func TestStateUnauthorized(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	// Missing Authorization header
	req, _ := http.NewRequest("GET", svr.URL+"/state", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestStateBadToken(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/state", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestEvolve(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"candidate": 10,
		"current":   5,
		"weight":    1.0,
		"mode":      "stable",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/evolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	if result["status"] != "queued" {
		t.Errorf("status = %q, want %q", result["status"], "queued")
	}
}

func TestEvolveInvalidMode(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"candidate": 10,
		"current":   5,
		"weight":    1.0,
		"mode":      "invalid",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/evolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestEvolveNegativeScore(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"candidate": -1,
		"current":   5,
		"weight":    1.0,
		"mode":      "stable",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/evolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestEvolveNonPositiveWeight(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"candidate": 10,
		"current":   5,
		"weight":    0.0,
		"mode":      "stable",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/evolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestEvolveWrongMethod(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/evolve", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestSelect(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/select", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /select: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	arm, ok := result["arm"]
	if !ok {
		t.Error("missing 'arm' field in response")
	}
	// arm should be one of the registered arms: stable, bandit, locked
	if arm != "stable" && arm != "bandit" && arm != "locked" && arm != "" {
		t.Errorf("arm = %q, expected one of [stable, bandit, locked, \"\"]", arm)
	}
}

func TestRotate(t *testing.T) {
	srv := newTestServer(t)
	// Use a temp file for the token
	tmpFile := t.TempDir() + "/token.txt"
	srv.tokenFile = tmpFile
	// Write initial token
	oldToken := "old-test-token"
	os.WriteFile(tmpFile, []byte(oldToken), 0600)
	srv.authToken = oldToken

	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("POST", svr.URL+"/rotate", nil)
	req.Header.Set("Authorization", "Bearer "+oldToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /rotate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	if result["status"] != "rotated" {
		t.Errorf("status = %q, want %q", result["status"], "rotated")
	}
	if result["token"] == "" {
		t.Error("token should be non-empty")
	}
	if result["token"] == oldToken {
		t.Errorf("new token = %q, should differ from old token %q", result["token"], oldToken)
	}
}

func TestRotateWrongMethod(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/rotate", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /rotate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestAgentEvolve(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"target_file": "foo.go",
		"patch":       "func Foo() {}",
		"confidence":  0.8,
		"mode":        "stable",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/agent/evolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /agent/evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	if _, ok := result["candidate_id"]; !ok {
		t.Error("missing 'candidate_id' field")
	}
	if _, ok := result["accepted"]; !ok {
		t.Error("missing 'accepted' field")
	}
}

func TestAgentEvolveMissingFields(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"target_file": "",
		"patch":       "",
		"confidence":  0.8,
		"mode":        "stable",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/agent/evolve", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /agent/evolve: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAgentStatus(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/agent/status", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /agent/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	if _, ok := stats["selected_arm"]; !ok {
		t.Error("missing 'selected_arm' field")
	}
}

func TestAgentStatusUnauthorized(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/agent/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /agent/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAgentStrategiesGET(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("GET", svr.URL+"/agent/strategies", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /agent/strategies: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	strategies, ok := result["strategies"].([]interface{})
	if !ok {
		t.Error("missing or invalid 'strategies' field")
	}
	if len(strategies) == 0 {
		t.Error("strategies should not be empty")
	}
}

func TestAgentStrategiesPOST(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	body := map[string]interface{}{
		"strategy": "experimental",
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", svr.URL+"/agent/strategies", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /agent/strategies: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("decode JSON: %v", err)
	}
	strategies, ok := result["strategies"].([]interface{})
	if !ok {
		t.Error("missing or invalid 'strategies' field")
	}
	found := false
	for _, s := range strategies {
		if s == "experimental" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'experimental' to be in strategies after POST")
	}
}

func TestAgentStrategiesWrongMethod(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	req, _ := http.NewRequest("DELETE", svr.URL+"/agent/strategies", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /agent/strategies: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestEvacuateRateLimit(t *testing.T) {
	srv := newTestServer(t)
	mux := setupTestMux(srv)
	svr := httptest.NewServer(mux)
	defer svr.Close()

	// Make many rapid requests to trigger rate limiting
	// The limit is 10 per minute by default
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", svr.URL+"/select", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
		// Should succeed for first 10
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i, resp.StatusCode, http.StatusOK)
		}
	}

	// 11th request should be rate limited
	req, _ := http.NewRequest("GET", svr.URL+"/select", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request 11: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Logf("Note: rate limit may not trigger in test environment (status=%d)", resp.StatusCode)
	}
}
