package evolution_test

import (
"bytes"
"encoding/json"
"net/http"
"net/http/httptest"
"strings"
"testing"

"pads-v3/internal/policy/evolution"
)

func setupTestServer() (*httptest.Server, *evolution.EventQueue) {
queue, _ := evolution.NewEventQueue("test_queue.log")
orch := evolution.NewOrchestrator(
evolution.NewMultiCycleEvaluator(),
evolution.NewStabilityGate(),
)
es := evolution.NewEventStore("test_ev.log")
wal := evolution.NewWAL()
detector := evolution.NewAntiCollapseDetector(5, 10.0)
bandit := evolution.NewBandit()
loop := evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, bandit)

worker := evolution.NewWorker(queue, loop, evolution.DeltaRewarder{})
go worker.Start()

// Simuler le serveur avec les handlers de l'API
mux := http.NewServeMux()
s := &server{queue: queue, worker: worker, authToken: "testtoken"}
mux.HandleFunc("/evolve", s.authMiddleware(s.enqueueEvolve))
mux.HandleFunc("/state", s.authMiddleware(s.state))
mux.HandleFunc("/health", s.health)

ts := httptest.NewServer(mux)
return ts, queue
}

type server struct {
queue     *evolution.EventQueue
worker    *evolution.Worker
authToken string
}

func (s *server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
authHeader := r.Header.Get("Authorization")
if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != s.authToken {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}
next(w, r)
}
}

func (s *server) enqueueEvolve(w http.ResponseWriter, r *http.Request) {
// Même code que dans main.go
var req struct {
Candidate int     `json:"candidate"`
Current   int     `json:"current"`
Weight    float64 `json:"weight"`
Mode      string  `json:"mode"`
}
json.NewDecoder(r.Body).Decode(&req)
event := evolution.QueueEvent{
ID:        "test",
Type:      "evolve",
Candidate: req.Candidate,
Current:   req.Current,
Weight:    req.Weight,
Mode:      evolution.Mode(req.Mode),
}
s.queue.Enqueue(event)
w.WriteHeader(http.StatusAccepted)
w.Write([]byte(`{"status":"queued"}`))
}

func (s *server) state(w http.ResponseWriter, r *http.Request) {
events, _ := s.queue.LoadAll()
var evs []evolution.Event
for i, e := range events {
evs = append(evs, evolution.Event{
Sequence:       i + 1,
CandidateScore: e.Candidate,
CurrentScore:   e.Current,
Weight:         e.Weight,
Mode:           e.Mode,
})
}
engine := evolution.NewReplayEngine(evs)
state := engine.Rebuild()
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(state)
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
w.Write([]byte("OK"))
}

func TestIntegrationFullCycle(t *testing.T) {
ts, _ := setupTestServer()
defer ts.Close()

// Étape 1 : POST /evolve
payload := `{"candidate":90,"current":50,"weight":1.0,"mode":"stable"}`
req, _ := http.NewRequest("POST", ts.URL+"/evolve", bytes.NewBufferString(payload))
req.Header.Set("Authorization", "Bearer testtoken")
req.Header.Set("Content-Type", "application/json")
resp, err := http.DefaultClient.Do(req)
if err != nil {
t.Fatal(err)
}
if resp.StatusCode != http.StatusAccepted {
t.Errorf("expected 202, got %d", resp.StatusCode)
}

// Étape 2 : GET /state
req2, _ := http.NewRequest("GET", ts.URL+"/state", nil)
req2.Header.Set("Authorization", "Bearer testtoken")
resp2, err := http.DefaultClient.Do(req2)
if err != nil {
t.Fatal(err)
}
if resp2.StatusCode != http.StatusOK {
t.Errorf("expected 200, got %d", resp2.StatusCode)
}
var state evolution.SystemState
json.NewDecoder(resp2.Body).Decode(&state)
if state.Sequence == 0 {
t.Error("expected non-zero sequence")
}
}
