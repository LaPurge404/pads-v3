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
	"pads-v3/internal/health"
	"pads-v3/internal/policy/evolution"
)

var (
	tokenFile = flag.String("token-file", "token.txt", "Fichier contenant le token d'authentification")
	certFile  = flag.String("cert", "", "Certificat TLS")
	keyFile   = flag.String("key", "", "Clé TLS")
	timeout   = flag.Duration("timeout", 30*time.Second, "Timeout pour les handlers HTTP")
)

type Server struct {
	tokenFile string
	queue     *evolution.EventQueue
	worker    *evolution.Worker
	authToken string
	rl        *evolution.RateLimiter
	selector  evolution.Selector
	pool      *agent.AgentPool // optional multi-agent pool
}

// securityHeaders ajoute les headers de sécurité sur toutes les réponses.
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

	selector := evolution.NewUCBSelector(time.Now().UnixNano())
	selector.AddArm("stable")
	selector.AddArm("bandit")
	selector.AddArm("locked")

	rewarder := evolution.DeltaRewarder{}

	loop := buildSafeLoop(selector)
	worker := evolution.NewWorker(queue, loop, rewarder)
	go worker.Start()

	srv := &Server{
		tokenFile: *tokenFile,
		queue:     queue,
		worker:    worker,
		authToken: authToken,
		rl:        evolution.NewRateLimiter(10, 1*time.Minute),
		selector:  selector,
	}

	// ── Router ─────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Public endpoints (pas de auth, mais avec security headers)
	mux.HandleFunc("/", srv.dashboard)
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/dashboard/enriched", srv.dashboardEnriched)

	// Protected endpoints: auth → rate limit → logging → handler
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

// authMiddleware — authentification Bearer token.
// L'ordre dans le chain est: securityHeaders → LoggingMiddleware → authMiddleware → RateLimiterMiddleware → handler
// On ne vérifie PAS le token pour /health (liveness).
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// /health ne nécessite pas d'auth
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

// handleRotate génère un nouveau token et le persiste dans token.txt.
// Nécessite une authentification.
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
	h := health.Check()

	// Add AgentPool stats if pool is configured.
	if s.pool != nil {
		poolStats := &health.PoolStats{
			Size: s.pool.Len(),
		}
		if best := s.pool.BestResult(); best != nil {
			poolStats.BestArm = best.UCBArm
		}
		poolStats.ArmStats = s.pool.PoolStats()
		h = health.CheckWithPool(poolStats)
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
