package main

import (
"crypto/rand"
"encoding/hex"
"encoding/json"
"flag"
"log/slog"
"net/http"
"os"
"strings"
"time"

"pads-v3/internal/policy/evolution"
)

var (
token    = flag.String("token", "", "Token d'authentification (généré si vide)")
certFile = flag.String("cert", "", "Certificat TLS")
keyFile  = flag.String("key", "", "Clé TLS")
)

type Server struct {
	queue     *evolution.EventQueue
	worker    *evolution.Worker
	authToken string
	rl        *evolution.RateLimiter
	selector  evolution.Selector
}

func main() {
	flag.Parse()

	// Token : flag, puis variable d'environnement, puis fichier token.txt, sinon génération
authToken := *token
if authToken == "" {
authToken = os.Getenv("PADS_TOKEN")
}
if authToken == "" {
data, err := os.ReadFile("token.txt")
if err == nil {
authToken = strings.TrimSpace(string(data))
}
}
if authToken == "" {
tok := make([]byte, 16)
rand.Read(tok)
authToken = hex.EncodeToString(tok)
slog.Info("Token généré", "token", authToken)
}

queue, err := evolution.NewEventQueue("event_queue.log")
if err != nil {
slog.Error("création queue", "error", err)
os.Exit(1)
}

selector := evolution.NewUCBSelector(time.Now().UnixNano())
selector.AddArm("stable")
selector.AddArm("bandit")
selector.AddArm("locked")

rewarder := evolution.DeltaRewarder{}

loop := buildSafeLoop(selector)
worker := evolution.NewWorker(queue, loop, rewarder)
go worker.Start()

s := &Server{
queue:     queue,
worker:    worker,
authToken: authToken,
rl:        evolution.NewRateLimiter(10, 1*time.Minute),
selector:  selector,
}

mux := http.NewServeMux()
mux.HandleFunc("/", s.dashboard)
mux.HandleFunc("/health", s.health)
mux.HandleFunc("/evolve", s.authMiddleware(s.rl.Middleware(evolution.LoggingMiddleware(s.enqueueEvolve))))
mux.HandleFunc("/state", s.authMiddleware(s.rl.Middleware(evolution.LoggingMiddleware(s.state))))
mux.HandleFunc("/select", s.authMiddleware(s.rl.Middleware(evolution.LoggingMiddleware(s.handleSelect))))
mux.HandleFunc("/workspace", s.authMiddleware(s.rl.Middleware(evolution.LoggingMiddleware(s.workspace))))

addr := "127.0.0.1:8080"
if *certFile != "" && *keyFile != "" {
slog.Info("API sécurisée (TLS)", "addr", addr)
slog.Error("serveur TLS", "error", http.ListenAndServeTLS(addr, *certFile, *keyFile, mux))
} else {
slog.Info("API", "addr", addr)
slog.Error("serveur", "error", http.ListenAndServe(addr, mux))
}
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
authHeader := r.Header.Get("Authorization")
if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != s.authToken {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}
next(w, r)
}
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
html := dashboardHTML
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Write([]byte(html))
}

func (s *Server) enqueueEvolve(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}
if r.Body == nil {
http.Error(w, "Empty body", http.StatusBadRequest)
return
}
var req struct {
Candidate int     `json:"candidate"`
Current   int     `json:"current"`
Weight    float64 `json:"weight"`
Mode      string  `json:"mode"`
}
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "Invalid JSON", http.StatusBadRequest)
return
}
if req.Candidate < 0 || req.Current < 0 {
http.Error(w, "Candidate and current scores must be non-negative", http.StatusBadRequest)
return
}
if req.Weight <= 0 {
http.Error(w, "Weight must be positive", http.StatusBadRequest)
return
}
validModes := map[string]bool{"stable": true, "bandit": true, "locked": true}
if !validModes[req.Mode] {
http.Error(w, "Invalid mode (allowed: stable, bandit, locked)", http.StatusBadRequest)
return
}

event := evolution.QueueEvent{
ID:        generateID(),
Type:      "evolve",
Candidate: req.Candidate,
Current:   req.Current,
Weight:    req.Weight,
Mode:      evolution.Mode(req.Mode),
}
if err := s.queue.Enqueue(event); err != nil {
slog.Error("enqueue evolve", "error", err)
http.Error(w, "Internal error", http.StatusInternalServerError)
return
}
slog.Info("évolution mise en file", "id", event.ID)
w.WriteHeader(http.StatusAccepted)
json.NewEncoder(w).Encode(map[string]string{"status": "queued", "id": event.ID})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}
queueEvents, err := s.queue.LoadAll()
if err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
return
}
events := convertQueueEventsToEvents(queueEvents)
engine := evolution.NewReplayEngine(events)
sysState := engine.Rebuild()
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(sysState)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
w.Write([]byte("OK"))
}

func (s *Server) handleSelect(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}
arm := s.selector.Select()
json.NewEncoder(w).Encode(map[string]string{"arm": arm})
}

func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
gitBranch := getGitBranch()
gitStatus := getGitStatus()
testResults := getTestResults()

resp := map[string]interface{}{
"gitBranch":  gitBranch,
"gitStatus":  gitStatus,
"testPassed": testResults.passed,
"testFailed": testResults.failed,
"testTotal":  testResults.total,
}
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

func generateID() string {
b := make([]byte, 8)
rand.Read(b)
return hex.EncodeToString(b)
}

func buildSafeLoop(selector evolution.Selector) *evolution.SafeEvolutionLoopV3 {
orch := evolution.NewOrchestrator(
evolution.NewMultiCycleEvaluator(),
evolution.NewStabilityGate(),
)
es := evolution.NewEventStore("evolution.log")
wal := evolution.NewWAL()
detector := evolution.NewAntiCollapseDetector(5, 10.0)
return evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, selector)
}

func convertQueueEventsToEvents(qev []evolution.QueueEvent) []evolution.Event {
var events []evolution.Event
for i, qe := range qev {
ev := evolution.Event{
Sequence:       i + 1,
CandidateScore: qe.Candidate,
CurrentScore:   qe.Current,
Weight:         qe.Weight,
Mode:           qe.Mode,
BanditSeed:     0,
}
events = append(events, ev)
}
return events
}
