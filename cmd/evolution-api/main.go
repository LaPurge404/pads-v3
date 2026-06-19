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

	"pads-v3/internal/agent"
	"pads-v3/internal/autonomous"
	"pads-v3/internal/health"
	"pads-v3/internal/policy/evolution"
)

var (
	tokenFile   = flag.String("token-file", "token.txt", "Fichier contenant le token d'authentification")
	certFile    = flag.String("cert", "", "Certificat TLS")
	keyFile     = flag.String("key", "", "Clé TLS")
	timeout     = flag.Duration("timeout", 30*time.Second, "Timeout pour les handlers HTTP")
	autonomousFlag = flag.Bool("autonomous", false, "Activer le mode autonome (équivalent de PADS_AUTONOMOUS=true)")
	projectRoot = flag.String("project-root", ".", "Racine du projet à améliorer")
)

type Server struct {
	tokenFile      string
	queue          *evolution.EventQueue
	worker         *evolution.Worker
	authToken      string
	rl             *evolution.RateLimiter
	selector       evolution.Selector
	pool           *agent.AgentPool    // optional multi-agent pool
	agentLoop      *evolution.AgentLoop // evolution engine for autonomous mode
	autonomousMode *autonomous.Mode    // autonomous closed-loop driver
	projectRoot    string
}

// securityHeaders adds security headers to all responses.
func securityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Request-ID", generateID())
		next(w, r)
	}
}

func main() {
	flag.Parse()

	// ── Token : flag > env > fichier > génération ──────────────────────
	authToken := os.Getenv("PADS_TOKEN")
	if authToken == "" {
		data, err := os.ReadFile(*tokenFile)
		if err == nil {
			authToken = strings.TrimSpace(string(data))
		}
	}
	if authToken == "" {
		tok := make([]byte, 16)
		if _, err := rand.Read(tok); err != nil {
			slog.Error("génération token aléatoire", "error", err)
			os.Exit(1)
		}
		authToken = hex.EncodeToString(tok)
		if err := os.WriteFile(*tokenFile, []byte(authToken), 0600); err != nil {
			slog.Error("écriture token dans fichier", "error", err)
		}
		slog.Info("Token généré", "token", authToken)
	}

	queue, err := evolution.NewEventQueue("event_queue.log")
	if err != nil {
		slog.Error("création queue", "error", err)
		os.Exit(1)
	}

	selector := evolution.NewUCBSelector(time.Now().UnixNano(), "ucb_state.json")
	selector.AddArm("stable")
	selector.AddArm("bandit")
	selector.AddArm("locked")
	selector.EnableAutoSave(30 * time.Second)

	rewarder := evolution.DeltaRewarder{}

	loop := buildSafeLoop(selector)
	worker := evolution.NewWorker(queue, loop, rewarder)
	go worker.Start()

	agentLoop := evolution.NewAgentLoop(loop, selector, rewarder)

	autoMode := autonomous.New()
	// Enable from flag or environment variable.
	if *autonomousFlag || os.Getenv("PADS_AUTONOMOUS") == "true" {
		autoMode.Enable()
	}

	srv := &Server{
		tokenFile:      *tokenFile,
		queue:          queue,
		worker:         worker,
		authToken:      authToken,
		rl:             evolution.NewRateLimiter(10, 1*time.Minute),
		selector:       selector,
		agentLoop:      agentLoop,
		autonomousMode: autoMode,
		projectRoot:    *projectRoot,
	}

	// ── Router ─────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Public endpoints (no auth, but with security headers)
	mux.HandleFunc("/", srv.dashboard)
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/dashboard/enriched", srv.dashboardEnriched)

	// Protected endpoints: securityHeaders → auth → rate limit → logging → handler
	protected := func(path string, h http.HandlerFunc) {
		chain := securityHeaders(h)
		chain = srv.rl.Middleware(chain)
		chain = srv.authMiddleware(chain)
		chain = evolution.LoggingMiddleware(chain)
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
	protected("/autonomous/toggle", srv.handleAutonomousToggle)
	protected("/autonomous/run", srv.handleAutonomousRun)

	// ── Server avec timeout global ───────────────────────────────────────
	addr := "127.0.0.1:8080"
	handler := http.TimeoutHandler(mux, *timeout, "Request timed out")

	slog.Info("API", "addr", addr, "timeout", *timeout)
	if *certFile != "" && *keyFile != "" {
		slog.Info("API sécurisée (TLS)", "addr", addr)
		slog.Error("serveur TLS", "error", http.ListenAndServeTLS(addr, *certFile, *keyFile, handler))
	} else {
		slog.Error("serveur", "error", http.ListenAndServe(addr, handler))
	}
}

// authMiddleware handles Bearer token authentication.
// The chain order is: securityHeaders → LoggingMiddleware → authMiddleware → RateLimiterMiddleware → handler
// We do NOT verify the token for /health (liveness).
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// /health does not require auth
		if r.URL.Path == "/health" {
			next(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != s.authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handleRotate generates a new token and persists it in token.txt.
// Requires authentication.
func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}
	newToken := hex.EncodeToString(tok)

	if err := os.WriteFile(s.tokenFile, []byte(newToken), 0600); err != nil {
		slog.Error("écriture nouveau token", "error", err)
		http.Error(w, "Failed to persist token", http.StatusInternalServerError)
		return
	}

	s.authToken = newToken
	slog.Warn("Token rotaté", "token", newToken)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rotated", "token": newToken})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	html := dashboardHTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (s *Server) dashboardEnriched(w http.ResponseWriter, r *http.Request) {
	html := enrichedDashboardHTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (s *Server) enqueueEvolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, ok := parseAndValidateEvolve(r, w)
	if !ok {
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
	// Build real health checker with filesystem and worker verification.
	paths := health.Paths{
		WALPath: s.queue.Path(),
		SemDB:   s.projectRoot + "/.pads/semantic/semantic_memory.db",
	}
	workerFn := func() bool { return false }
	if s.worker != nil {
		workerFn = s.worker.IsRunning
	}
	checker := health.NewChecker(paths, workerFn)
	h := checker.Check()

	// Add AgentPool stats if pool is configured.
	if s.pool != nil {
		poolStats := &health.PoolStats{
			Size: s.pool.Len(),
		}
		if best := s.pool.BestResult(); best != nil {
			poolStats.BestArm = best.UCBArm
		}
		poolStats.ArmStats = s.pool.PoolStats()
		h.Pool = poolStats
	}

	// Add autonomous mode status.
	if s.autonomousMode != nil {
		h.Autonomous = &health.AutonomousStatus{
			Enabled: s.autonomousMode.IsEnabled(),
			Cycles:  s.autonomousMode.CycleNum(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h)
}

func (s *Server) handleSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	arm := s.selector.Select()
	json.NewEncoder(w).Encode(map[string]string{"arm": arm})
}

// handleAutonomousToggle flips the autonomous mode state.
// POST /autonomous/toggle — returns new state.
func (s *Server) handleAutonomousToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	enabled := s.autonomousMode.Toggle()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":        enabled,
		"cycles_total":   s.autonomousMode.CycleNum(),
		"note":           "autonomous mode is now "+map[bool]string{true: "ENABLED", false: "DISABLED"}[enabled],
	})
}

// handleAutonomousRun executes one autonomous improvement cycle.
// POST /autonomous/run — body: {"target": "path/to/file.go", "goal": "fix description"}
func (s *Server) handleAutonomousRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Target string `json:"target"`
		Goal   string `json:"goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Target == "" || req.Goal == "" {
		http.Error(w, "both 'target' and 'goal' are required", http.StatusBadRequest)
		return
	}

	// Lazy-initialize CodeAgent and SandboxExecutor per run.
	// This avoids carrying state across independent cycles.
	codeAgent := agent.NewCodeAgent(agent.NewDefaultLLMClient())
	sandboxExec := agent.NewSandboxExecutor(s.projectRoot, false)

	task := agent.Task{
		Kind:   agent.TaskFixBroken,
		Target: req.Target,
		Goal:   req.Goal,
	}

	result := s.autonomousMode.RunCycle(
		task,
		codeAgent,
		sandboxExec,
		s.agentLoop,
		s.projectRoot,
		0.0,       // semanticRisk: 0 (no analysis in this call)
		[]string{}, // semanticReasons
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		b[0] = byte(time.Now().UnixNano() & 0xff)
		b[1] = byte((time.Now().UnixNano() >> 8) & 0xff)
	}
	return hex.EncodeToString(b)
}

func buildSafeLoop(selector evolution.Selector) *evolution.SafeEvolutionLoopV3 {
	orch := evolution.NewOrchestrator(
		evolution.NewMultiCycleEvaluator(),
		evolution.NewStabilityGate(),
	)
	es := evolution.NewEventStore("evolution.log")
	wal := evolution.NewWAL("evolution.wal")
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
