# DESIGN.md

## 1. Architecture Overview (ASCII diagram)

```
EventQueue ──► Event ──► Worker ──► SafeEvolutionLoopV3 ──► EventStore
                 │                               │
                 └─────────────────► ReplayEngine ◄──┘
```

* **EventQueue** – serialises incoming events (`QueueEvent`) to a log file and provides an `Enqueue` method.  
* **Event** – Go struct defined in `internal/policy/evolution/event.go`; see section 2 for its fields.  
* **Worker** – reads events from the queue, passes them to `SafeEvolutionLoopV3`, and triggers the learning components.  
* **SafeEvolutionLoopV3** – core controller that runs, in order:  
  1. `MultiCycleEvaluator`  
  2. `StabilityGate`  
  3. `AntiCollapseDetector`  
  4. `RollbackManager` (internal rollback logic)  
  5. Persists the decision in `EventStore` and notifies `ReplayEngine`.  
* **EventStore** – durable storage (file‑based) that holds the complete event history and provides `LoadAll`, `InsertEvent`, `UpdateFileHash`.  
* **ReplayEngine** – consumes stored events to rebuild the system state on demand (`Rebuild` method).

---

## 2. Event Structure (Go)

The canonical event type is defined in `internal/policy/evolution/event.go`:

```go
type Event struct {
    Sequence       int                     // monotonically increasing sequence number
    CandidateScore int                     // score of the candidate to evolve
    CurrentScore   int                     // current score of the system
    Weight         float64                 // user supplied weight for the candidate
    Mode           Mode                    // evolution mode: stable|bandit|locked
    BanditSeed     int64                   // seed for bandit exploration
    GateVariance   float64                 // variance observed in the stability gate
    GateThreshold  float64                 // threshold used by the stability gate
    StabilityScore float64                 // latest stability score (0‑1)
    Trend          float64                 // slope of stability trend
    Reason         string                  // textual explanation of the decision
}
```

When persisted, the event is written to the log as a direct JSON representation of this struct (see section 3 for a concrete example).

---

## 3. HTTP API

All **protected** endpoints require an `Authorization: Bearer ***` header.  
The token can be supplied via the `PADS_TOKEN` environment variable, a `token.txt` file, or generated automatically on first start (see Security section).

| Method | URL | Description | Example `curl` (with token) | Expected response |
|--------|-----|-------------|-----------------------------|-------------------|
| `GET` | `/health` | Liveness probe – always returns plain text `OK`. | `curl -s http://127.0.0.1:8080/health` | `OK` (plain‑text) |
| `GET` | `/state` | Returns the full system state derived from all stored events **by rebuilding** with `ReplayEngine.Rebuild()`. | `curl -s -H "Authorization: Bearer *** http://127.0.0.1:8080/state"` | JSON serialization of `SystemState` (see `system_state.go`); e.g. a typical payload looks like `{"bandit":{},"gate":{},"detector_window":[],"mode":"stable","sequence":0,"stability_score":0,"trend":0,"reason":""}` |
| `POST` | `/evolve` | Submits a new evolution request. Body must contain `candidate`, `current`, `weight`, `mode`. The server validates, creates a `QueueEvent`, and enqueues it. | ```bash\ncurl -s -X POST http://127.0.0.1:8080/evolve \\\n  -H "Authorization: Bearer ***" \\\n  -H "Content-Type: application/json" \\\n  -d '{"candidate":12,"current":7,"weight":1.5,"mode":"stable"}'\n``` | `{"status":"queued","id":"<generated-id>"}` |
| `GET` | `/select` | Returns the arm chosen by the bandit selector for the next exploration step. | `curl -s -H "Authorization: Bearer *** http://127.0.0.1:8080/select"` | `{"arm":"stable"}` (or `"bandit"`/`"locked"` depending on selector state) |
| `GET` | `/workspace` | Shows repository metadata and test results used by the dashboard. | `curl -s -H "Authorization: Bearer *** http://127.0.0.1:8080/workspace"` | `{"gitBranch":"feature/closed-loop-autotune","gitStatus":"(propre)","testPassed":42,"testFailed":0}` |

*Note*: The `/workspace` endpoint is implemented in `cmd/evolution-api/workspace.go` and queries the git repository and runs `go test ./...` to count passed/failed packages.

---

## 4. Decision Flow (as executed by `SafeEvolutionLoopV3`)

1. **Enqueue** – `QueueEvent` is written to the log file via `EventQueue.Enqueue`.  
2. **Worker** picks the event and calls `SafeEvolutionLoopV3.Run`.  
3. `MultiCycleEvaluator` evaluates the candidate configuration (multi‑cycle simulation).  
4. **StabilityGate** checks whether the new stability score meets the configured threshold; if not, the candidate is rejected immediately.  
5. **AntiCollapseDetector** inspects recent trend and variance; a high risk of oscillation or drift triggers a *collapse* handling path.  
6. **RollbackManager** (internal) creates a temporary rollback plan if the detector signals instability; the plan is executed before the candidate proceeds.  
7. If the candidate passes the above checks, it is **accepted** and persisted as an `Event` in `EventStore`.  
8. The accepted event is then handed to **ReplayEngine** for possible time‑travel debugging.  
9. **Learning** – the selected arm (stable / bandit / locked) is updated via the configured **Rewarder** (normally `DeltaRewarder`) and the **UCBSelector** which computes the next selection probability based on empirical rewards.

---

## 5. Security

* **Token handling** –  
  1. The server parses a command‑line flag (`-token`).  
  2. If empty, it reads the environment variable `PADS_TOKEN`.  
  3. If still empty, it looks for `$HOME/.pads/token.txt`; the file content (trimmed) becomes the token.  
  4. If no token is found, a fresh 128‑bit token is generated, stored in `token.txt`, and used for the session.  
  5. Every protected request must contain `Authorization: Bearer ***`; any deviation yields HTTP 401.  

* **Rate limiting** – a token‑bucket limiter instantiated as `evolution.NewRateLimiter(10, 1*time.Minute)` allows **10 requests per minute** globally. Exceeding the quota returns HTTP 429 with a `Retry-After` header.  

* **TLS** – optional. When the flags `-cert <path>` and `-key <path>` are supplied the HTTP server starts with `ListenAndServeTLS`; otherwise it uses plain `ListenAndServe`. No TLS termination proxy is required.

---

## 6. Learning (Bandit & Rewarder)

* **Rewarder interface** ( `internal/policy/evolution/rewarder.go` ):

  ```go
  type Rewarder interface {
      ComputeReward(oldStability, newStability float64, accepted bool) float64
  }
  ```

  The default production implementation is **`DeltaRewarder`**:

  ```go
  type DeltaRewarder struct{}
  func (d DeltaRewarder) ComputeReward(oldStability, newStability float64, accepted bool) float64 {
      // reward = delta stability if accepted, otherwise 0
      if !accepted {
          return 0
      }
      return newStability - oldStability
  }
  ```

* **UCB bandit** – defined in `internal/policy/evolution/bandit_ucb.go`.  
  Core type:

  ```go
  type UCBSelector struct {
      seed       int64
      arms       map[string]*arm   // arm contains currentReward, pulls
      pullCounts map[string]int
  }
  ```

  * `AddArm(name string)` – registers a new arm (the system pre‑populates `stable`, `bandit`, `locked`).  
  * `Update(name string, reward float64)` – stores the reward and increments the pull counter.  
  * `Select() string` – returns the arm with the highest **UCB1** value:  

    `score = meanReward + sqrt( log(totalPulls) / armPulls )`

* **Learning loop** – after a candidate is accepted the `SafeEvolutionLoopV3` calls the configured `Rewarder.ComputeReward` with the stability before and after the operation and whether the operation was accepted. The returned reward is fed to `UCBSelector.Update`. The next call to `/select` reads the arm chosen by `UCBSelector.Select()` and returns it as JSON.

---

## 7. Summary of Real Implementation Details

* The system is **event‑driven**: an HTTP POST to `/evolve` only enqueues a lightweight descriptor; the heavy execution happens later in the worker and loop.  
* The **event struct** contains many more fields than a simple `id/type/payload` model; they are required for stability analysis and rollback decisions.  
* HTTP responses are **minimal JSON objects**; the `/state` payload is generated dynamically by the `ReplayEngine.Rebuild` method and reflects the `SystemState` fields.  
* **Token** generation and storage are fully handled inside `main.go`; the server never reads a static configuration file other than `token.txt`.  
* **Rate limiting** is a hard limit of 10 req/min enforced by a dedicated limiter; it is independent of the token check.  
* **Learning components** are wired directly in `main.go`: a `UCBSelector` with three predefined arms and a `DeltaRewarder` are instantiated once at startup and passed to the `Worker`.  
* The **ReplayEngine** is only used for the `/state` endpoint (to rebuild the full system state) and for manual debugging; it does **not** influence the normal evolution path.

All sections above now reflect the **actual code** located in `~/workspace/pads-v3`. No speculative or outdated information remains.

---

## 8. CodeAgent Architecture (New in agent/hermes-dev)

### Overview

The CodeAgent is an autonomous code improvement agent that uses an LLM to generate code modifications. It is integrated with the evolution engine and follows the supervised闭环 loop pattern.

```
CodeAgent → LLM → Plan → Executor → EvolutionEngine → UCB Learning
```

### Components

**LLMClient Interface** (`internal/agent/llm.go`):
```go
type LLMClient interface {
    GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error)
}
```

Implemented by:
- `OpenAIClient` - OpenAI API integration (set `OPENAI_API_KEY`)
- `ClaudeClient` - Anthropic Claude API (set `ANTHROPIC_API_KEY`)

**CodeAgent** (`internal/agent/code_agent.go`):
```go
type CodeAgent struct {
    llm           LLMClient
    executor      *Executor
    maxRetries    int
    minConfidence float64
}
```

Features:
- Builds context from storage (L2 events, L3 state)
- Generates code via LLM with confidence scoring
- Produces executable Plans with write_file and run_command actions
- Respects minimum confidence threshold (default 0.6)

**ContextBuilder** (`internal/agent/context_builder.go`):
Enriches the agent context with:
- Package path from nodes table
- Node ID from graph_state
- Recent L2 events for the target file
- Aggregated L3 state summary (total/broken/stable counts)

### Agent Workflow

1. `BuildTasks(db)` - Find BROKEN nodes, create Tasks
2. `BuildContext(db, task)` - Enrich context with storage data
3. `CodeAgent.Solve(task, ctx)` - Query LLM for code fix
4. `Executor.ExecutePlan(plan)` - Apply changes
5. Evolution engine evaluates the change via UCB

### Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key for GPT models |
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude |
| `OPENAI_BASE_URL` | Custom OpenAI-compatible endpoint |

---  
*End of corrected DESIGN.md*