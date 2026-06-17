# DESIGN.md

## 1. Architecture Overview (ASCII diagram)

```
API ──► Event Queue ──► Worker ──► SafeEvolutionLoopV3 ──► EventStore
                                   │                       │
                                   └─────────────► ReplayEngine ◄──┘
```

## 2. Event Format (Go)

```go
type Event struct {
    ID        string        `json:"id"`
    Type      string        `json:"type"`      // "evolve", "metric", "state"
    Payload   []byte        `json:"payload"`   // raw JSON payload
    Timestamp time.Time     `json:"timestamp"`
}
```

**Example JSON event**

```json
{
  "id": "engine-5f4e2a1b",
  "type": "evolve",
  "payload": "{\"config\":{\"rate\":0.75,\"model\":\"gpt-3.5\"}",
  "timestamp": "2026-06-18T12:34:56Z"
}
```

## 3. HTTP Endpoints

| Method | URL               | curl example (with token)                                    | Expected response (excerpt) |
|--------|-------------------|--------------------------------------------------------------|-----------------------------|
| GET    | `/health`         | `curl -s http://127.0.0.1:8080/health`                         | `{"status":"ok"}`            |
| GET    | `/state`          | `curl -s -H "Authorization: Bearer $PADS_TOKEN" http://127.0.0.1:8080/state` | `{"state":"running","queued":12}` |
| POST   | `/evolve`         | `curl -s -X POST http://127.0.0.1:8080/evolve \\\n  -H "Authorization: Bearer $PADS_TOKEN" \\\n  -H "Content-Type: application/json" \\\n  -d '{"action":"start","params":{"model":"gpt-4","iterations":3}}'` | `{"evolution_id":"evo-7a9c","status":"queued"}` |
| GET    | `/select`         | `curl -s -H "Authorization: Bearer $PADS_TOKEN" http://127.0.0.1:8080/select` | `{"selected":"config-v2"}` |
| GET    | `/workspace`      | `curl -s -H "Authorization: Bearer $PADS_TOKEN" http://127.0.0.1:8080/workspace` | `{"workspace":"default","files":["main.go","config.yaml"]}` |

*All protected endpoints require the `PADS_TOKEN` environment variable (or the file `$HOME/.pads/token.txt`).*

## 4. Decision Flow

```
Submission (POST /evolve)
      │
      ▼
Evaluation  →  StabilityGate (adaptive threshold)
      │
      ▼
Anti‑CollapseDetector (oscillation / drift check)
      │
      ▼
RollbackManager (undo unsafe changes if needed)
      │
      ├─► Acceptance → Event queued → SafeEvolutionLoopV3 executes
      └─► Rejection  → Immediate response (403) + learning update
      │
      ▼
Learning Update (UCB / Rewarder) → adjusts arm probabilities
```

## 5. Security

- **Token mechanism**  
  - At startup a random token is generated and stored in `$HOME/.pads/token.txt`.  
  - The token can be overridden by setting the environment variable `PADS_TOKEN`.  
  - The token is sent in the `Authorization: Bearer <token>` header for every protected endpoint.

- **Rate limiting**  
  - A token‑bucket limiter (`limit: 30 req/min`) is applied globally.  
  - Exceeding the limit returns HTTP 429 with `Retry-After` header.

- **TLS**  
  - By default the HTTP server listens on plain `localhost:8080`.  
  - TLS can be enabled with flags `-cert <path> -key <path>`; when enabled the server redirects HTTP → HTTPS.

## 6. Learning

- **Rewarder**  
  - Each arm (e.g., a model configuration) receives a scalar reward based on outcome (success, failure, rollback).  
  - Rewards are accumulated per‑arm over sliding windows.

- **UCB (Upper Confidence Bound)**  
  - The selection score for arm *i* is `θ_i + α * sqrt(log(N) / n_i)`  
    - `θ_i` = empirical mean reward of arm *i*  
    - `α` = exploration constant (default = 1.0)  
    - `N` = total pulls, `n_i` = pulls of arm *i*  
  - Arms with higher uncertainty get explored more frequently.

- **Arm update**  
  - After each evolution attempt, the reward is computed (e.g., +1 for successful deployment, –1 for rollback).  
  - `θ_i` and `n_i` are updated, and the next selection probability is recomputed.

---

*End of DESIGN.md*