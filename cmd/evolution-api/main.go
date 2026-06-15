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
}

func main() {
flag.Parse()

authToken := *token
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

loop := buildSafeLoop()
worker := evolution.NewWorker(queue, loop)
go worker.Start()

s := &Server{
queue:     queue,
worker:    worker,
authToken: authToken,
rl:        evolution.NewRateLimiter(10, 1*time.Minute),
}

mux := http.NewServeMux()
mux.HandleFunc("/", s.dashboard)
mux.HandleFunc("/health", s.health)
mux.HandleFunc("/evolve", s.authMiddleware(s.rl.Middleware(s.enqueueEvolve)))
mux.HandleFunc("/state", s.authMiddleware(s.rl.Middleware(s.state)))

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
html := `<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PADS Evolution Control</title>
    <style>
        :root {
            --bg: #f5f7fa;
            --card-bg: #ffffff;
            --text: #2d3748;
            --primary: #3182ce;
            --success: #38a169;
            --danger: #e53e3e;
            --border: #e2e8f0;
        }
        body {
            font-family: 'Segoe UI', system-ui, sans-serif;
            background: var(--bg);
            margin: 0;
            padding: 20px;
            color: var(--text);
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
        }
        h1 {
            margin-bottom: 10px;
            font-weight: 600;
        }
        .card {
            background: var(--card-bg);
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.05);
        }
        .token-section {
            display: flex;
            gap: 10px;
            align-items: center;
            margin-bottom: 15px;
        }
        input, select, button {
            padding: 10px 15px;
            border: 1px solid var(--border);
            border-radius: 8px;
            font-size: 14px;
        }
        button {
            background: var(--primary);
            color: white;
            border: none;
            cursor: pointer;
            font-weight: 500;
        }
        button:hover { opacity: 0.9; }
        .status-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
        }
        .stat {
            background: var(--bg);
            border-radius: 8px;
            padding: 15px;
            text-align: center;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: var(--primary);
        }
        canvas {
            width: 100%;
            max-height: 200px;
            background: var(--bg);
            border-radius: 8px;
        }
        .error { color: var(--danger); }
        .success { color: var(--success); }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧬 PADS Evolution Control</h1>
        <div class="card">
            <h3>🔑 Authentification</h3>
            <div class="token-section">
                <input type="password" id="tokenInput" placeholder="Token d'accès" style="flex:1">
                <button onclick="saveToken()">Enregistrer</button>
            </div>
            <p><small>Le token est stocké localement (localStorage). Il est envoyé avec chaque requête.</small></p>
        </div>

        <div class="card">
            <h3>⚡ Envoyer une nouvelle évolution</h3>
            <form id="evolveForm">
                <div style="display:grid; grid-template-columns:1fr 1fr; gap:10px;">
                    <input type="number" id="candidate" placeholder="Score candidat" required>
                    <input type="number" id="current" placeholder="Score courant" required>
                    <input type="number" id="weight" placeholder="Poids" step="0.1" value="1.0">
                    <select id="mode">
                        <option value="stable" selected>Stable</option>
                        <option value="bandit">Bandit</option>
                        <option value="locked">Locked</option>
                    </select>
                </div>
                <button type="submit" style="margin-top:10px;">🚀 Lancer l'évolution</button>
            </form>
        </div>

        <div class="card">
            <h3>📊 État actuel du système</h3>
            <div class="status-grid" id="statusGrid">
                <div class="stat"><div class="stat-value" id="modeVal">-</div>Mode</div>
                <div class="stat"><div class="stat-value" id="seqVal">-</div>Séquence</div>
                <div class="stat"><div class="stat-value" id="varianceVal">-</div>Variance</div>
                <div class="stat"><div class="stat-value" id="thresholdVal">-</div>Seuil adaptatif</div>
            </div>
            <canvas id="scoreChart" style="margin-top:20px;"></canvas>
        </div>
        <div id="message"></div>
    </div>

    <script>
        function getToken() {
            return localStorage.getItem('evolutionToken') || '';
        }

        function saveToken() {
            const token = document.getElementById('tokenInput').value.trim();
            if (token) {
                localStorage.setItem('evolutionToken', token);
                document.getElementById('tokenInput').value = '';
                showMessage('Token enregistré.', 'success');
                refreshState();
            }
        }

        window.onload = () => {
            const stored = getToken();
            if (stored) {
                document.getElementById('tokenInput').placeholder = 'Token chargé depuis le stockage';
            }
            refreshState();
        };

        async function refreshState() {
            const token = getToken();
            if (!token) return;
            try {
                const resp = await fetch('/state', {
                    headers: { 'Authorization': 'Bearer ' + token }
                });
                if (resp.ok) {
                    const state = await resp.json();
                    updateUI(state);
                } else {
                    showMessage('État non autorisé (vérifiez le token)', 'error');
                }
            } catch(e) {
                showMessage('Erreur réseau', 'error');
            }
        }

        function updateUI(state) {
            document.getElementById('modeVal').textContent = state.Mode;
            document.getElementById('seqVal').textContent = state.Sequence;
            const window = state.DetectorWindow || [];
            if (window.length > 1) {
                const mean = window.reduce((a,b) => a+b, 0) / window.length;
                const variance = window.reduce((a,b) => a + (b-mean)**2, 0) / window.length;
                document.getElementById('varianceVal').textContent = variance.toFixed(2);
            } else {
                document.getElementById('varianceVal').textContent = '0';
            }
            document.getElementById('thresholdVal').textContent = (state.Gate.Threshold || 0).toFixed(2);
            drawChart(window);
        }

        function drawChart(data) {
            const canvas = document.getElementById('scoreChart');
            const ctx = canvas.getContext('2d');
            canvas.width = canvas.clientWidth;
            canvas.height = 200;
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            if (!data.length) return;
            const step = canvas.width / (data.length - 1 || 1);
            const maxVal = Math.max(...data) || 1;
            ctx.beginPath();
            ctx.strokeStyle = '#3182ce';
            ctx.lineWidth = 2;
            data.forEach((val, i) => {
                const x = i * step;
                const y = canvas.height - (val / maxVal) * (canvas.height - 20) - 10;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();
        }

        function showMessage(msg, type) {
            const div = document.getElementById('message');
            div.textContent = msg;
            div.className = type;
            setTimeout(() => { div.textContent = ''; }, 4000);
        }

        document.getElementById('evolveForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const token = getToken();
            if (!token) {
                showMessage('Veuillez d\'abord enregistrer le token.', 'error');
                return;
            }
            const body = {
                candidate: parseInt(document.getElementById('candidate').value),
                current: parseInt(document.getElementById('current').value),
                weight: parseFloat(document.getElementById('weight').value),
                mode: document.getElementById('mode').value
            };
            try {
                const resp = await fetch('/evolve', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + token
                    },
                    body: JSON.stringify(body)
                });
                if (resp.ok) {
                    const data = await resp.json();
                    showMessage('Évolution envoyée (ID: ' + data.id + ')', 'success');
                    setTimeout(refreshState, 500);
                } else {
                    showMessage('Erreur lors de l\'envoi', 'error');
                }
            } catch(err) {
                showMessage('Erreur réseau', 'error');
            }
        });

        setInterval(refreshState, 2000);
    </script>
</body>
</html>`
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Write([]byte(html))
}

// POST /evolve avec validation et rate limiting déjà appliqués
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

func generateID() string {
b := make([]byte, 8)
rand.Read(b)
return hex.EncodeToString(b)
}

func buildSafeLoop() *evolution.SafeEvolutionLoopV3 {
orch := evolution.NewOrchestrator(
evolution.NewMultiCycleEvaluator(),
evolution.NewStabilityGate(),
)
es := evolution.NewEventStore("evolution.log")
wal := evolution.NewWAL()
detector := evolution.NewAntiCollapseDetector(5, 10.0)
bandit := evolution.NewBandit()
return evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, evolution.ModeStable, bandit)
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
