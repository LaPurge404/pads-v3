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
| `GET` | `/health` | Liveness probe – returns a JSON `HealthChecker` with fields `db`, `wal`, `semantic_memory`, `worker`, and optionally `pool` and `autonomous`. | `curl -s http://127.0.0.1:8080/health` | JSON, e.g. `{"db":true,"wal":true,"semantic_memory":true,"worker":true}` |
| `GET` | `/metrics` | Prometheus metrics endpoint. **Public** (no auth required). Returns counters `evolution_cycles_total`, `ucb_updates_total`, `sandbox_runs_total`, `autonomous_cycles_total`, `worker_errors_total`. | `curl -s http://127.0.0.1:8080/metrics` | Prometheus text exposition format |
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

* **Token handling** – Token resolution order: 1) `PADS_TOKEN` environment variable; 2) `token.txt` file in the working directory (configurable via `-token-file`); 3) auto-generated 128-bit random token written to `token.txt`. The flag `-token` is deprecated. Every protected request must contain `Authorization: Bearer ***` — any deviation yields HTTP 401.

* **Token rotation** – `POST /rotate` (authenticated) generates a new token, writes it to `token.txt`, and returns it in the response.

* **Rate limiting** – a token-bucket limiter (`evolution.NewRateLimiter(10, 1*time.Minute)`) allows **10 requests per minute** per distinct Bearer token. A background goroutine cleans up stale/inactive tokens every 5 minutes and enforces a max of 1000 distinct tokens (LRU eviction). Exceeding the quota returns HTTP 429 with a `Retry-After` header.

* **Middleware chain order** – Rate limiting is checked **before** auth (rate limiter is closest to the handler): `securityHeaders → rateLimiterMiddleware → authMiddleware → LoggingMiddleware → handler`. This means unauthenticated requests also consume rate limit quota — a deliberate choice to protect against unauthenticated DoS floods. The `/metrics` endpoint is public (no auth) but retains security headers.

* **HTTP security headers** – Every response includes `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `X-Request-ID`.

* **Request timeout** – All handlers are wrapped in `http.TimeoutHandler` (default: 30 seconds, configurable via `-timeout`). Requests exceeding the timeout receive a `"Request timed out"` response.

* **TLS** – optional. When `-cert <path>` and `-key <path>` are supplied the server uses `ListenAndServeTLS`; otherwise plain `ListenAndServe`. For local testing: generate a self-signed cert with `openssl req -new -x509 -key server.key -out server.crt -days 365 -subj "/CN=localhost"`.

* **WAL persistence** – The Write-Ahead Log (`evolution.wal`) is fsync'd to disk on every append. On restart, entries are reloaded from disk. File format: one JSON line per entry.

* **LLM API keys** – `NVIDIA_API_KEY` (default), `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` — read from environment, never hardcoded.

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
| `NVIDIA_API_KEY` | NVIDIA NIM API key (default LLM provider) |
| `OPENAI_API_KEY` | OpenAI API key for GPT models |
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude |
| `OPENAI_BASE_URL` | Custom OpenAI-compatible endpoint |

---

## 9. Sandbox Executor (Phase 2)

### Overview

The Sandbox provides an isolated environment to test code changes before applying them to the production codebase. It follows a **copy-test-apply** pattern:

```
Plan → Sandbox Copy → Apply Changes → Run Tests
                                     ├── Pass  → Apply to Real FS
                                     └── Fail  → Rollback (no changes)
```

### Components

**Sandbox** (`internal/agent/sandbox.go`):
```go
type Sandbox struct {
    workDir     string  // temp directory with project copy
    projectRoot string  // original project path
    cleanup     func()  // cleanup function
}
```

Key methods:
- `NewSandbox(projectRoot)` - Creates isolated copy
- `ApplyChange(targetPath, content)` - Applies change to sandbox copy
- `RunTests()` - Executes test suite in sandbox
- `Close()` - Cleans up temp directory

**SandboxExecutor** (`internal/agent/sandbox.go`):
```go
type SandboxExecutor struct {
    executor    *Executor
    projectRoot string
    autoCleanup bool
}
```

Key methods:
- `ExecuteWithSandbox(plan)` - Runs plan in sandbox, applies if tests pass, rolls back if fail
- `DryRunWithSandbox(plan)` - Tests without applying changes

### Sandbox Workflow

1. `NewSandbox(root)` creates a temp copy of the project (excluding `.git`)
2. Plan actions modify the **sandbox copy** only
3. `RunTests()` executes the full test suite in sandbox
4. If tests **pass**: changes are applied to the **real** filesystem
5. If tests **fail**: sandbox is discarded, original is **unchanged**
6. Temp directory is cleaned up

### Safety Properties

- Original files are **never** modified during sandbox execution
- Changes only persist to the real filesystem after tests pass
- Build errors are caught before any modification
- Automatic cleanup of temp directories

---

## 10. CodeAgent ↔ Evolution Engine Integration (Phase 3)

### Overview

The CodeAgent generates code modifications that are evaluated by the SafeEvolutionLoopV3 engine. The UCB bandit selector learns which agent prompting strategies produce the best outcomes.

### Architecture

```
CodeAgent (LLM: Nvidia by default)
    ↓ generates Plan
SandboxExecutor
    ↓ validates (tests pass/fail)
AgentCandidate → SafeEvolutionLoopV3
    ↓ UCB feedback
UCBSelector (learns best strategies)
```

### Components

**AgentCandidate** (`internal/policy/evolution/agent_loop.go`):
```go
type AgentCandidate struct {
    ID         string
    TargetFile string
    Patch      string
    Confidence float64  // LLM confidence (0-1)
    Retries    int
    Strategy   string   // UCB arm name
    CreatedAt  time.Time
}
```

**AgentResult** (`internal/policy/evolution/agent_loop.go`):
```go
type AgentResult struct {
    CandidateID    string
    Score          int           // Evolution score after evaluation
    Accepted       bool          // Whether evolution accepted the candidate
    CycleResult    CycleResult
    StabilityScore float64
    Reason         string        // Human-readable explanation
    UCBArm         string        // Strategy used
    Reward         float64       // UCB reward computed
}
```

**AgentLoop** (`internal/policy/evolution/agent_loop.go`):
```go
type AgentLoop struct {
    loop     *SafeEvolutionLoopV3
    selector *UCBSelector
    rewarder Rewarder
}
```

Key methods:
- `Evaluate(candidate, currentScore, weight)` - Evaluates candidate, updates UCB
- `SelectArm()` - Returns current UCB-selected strategy
- `AddArm(name)` - Registers a new strategy arm
- `UCBStats()` - Returns statistics for all arms

**EvolutionConnector** (`internal/agent/evolution_bridge.go`):
```go
type EvolutionConnector struct {
    codeAgent    *CodeAgent
    sandboxExec  *SandboxExecutor
    agentLoop    *evolution.AgentLoop
    currentScore int
}
```

Key methods:
- `SuggestAndEvaluate(task, ctx)` - Full pipeline: generate → sandbox → evolve → learn

**CodeAgentForEvolution** (`internal/agent/evolution_bridge.go`):
```go
type CodeAgentForEvolution struct {
    CodeAgent    *CodeAgent
    SandboxExec  *SandboxExecutor
    AgentLoop    *evolution.AgentLoop
    CurrentScore int
    ProjectRoot  string
}
```

Key methods:
- `RunTask(task, ctx)` - Runs a task through the full evolution pipeline
- `FixBrokenNode(ctx)` - Convenience method for fixing broken nodes

### LLM Providers

**NvidiaClient** (`internal/agent/llm.go`) is the **default** LLM provider:
- Uses `NVIDIA_API_KEY` environment variable
- Default model: `meta/llama-3.1-70b-instruct`
- Customizable via `NVIDIA_BASE_URL` and `NVIDIA_MODEL` env vars

Other providers available but not default:
- `OpenAIClient` - uses `OPENAI_API_KEY`
- `ClaudeClient` - uses `ANTHROPIC_API_KEY`

Factory functions:
- `NewDefaultLLMClient()` - Returns Nvidia client (recommended)
- `NewCodeAgentDefault()` - Creates CodeAgent with Nvidia
- `NewCodeAgentForEvolutionDefault(projectRoot, agentLoop)` - Full pipeline with Nvidia

### API Endpoints

**POST /agent/evolve** - Submit an agent candidate for evolution evaluation:
```json
{
  "target_file": "internal/foo/bar.go",
  "patch": "func Hello() { return \"world\" }",
  "confidence": 0.85,
  "mode": "stable"
}
```

Response:
```json
{
  "candidate_id": "abc123",
  "accepted": true,
  "score": 72,
  "confidence": 0.85,
  "stability_score": 0.72,
  "reason": "Accepted: improvement +22",
  "ucb_arm": "conservative",
  "reward": 0.22,
  "sandbox_passed": true
}
```

**GET /agent/status** - Returns UCB statistics and selected arm.

**GET/POST /agent/strategies** - List or register agent strategies for UCB selection.

### Workflow

1. **CodeAgent.Solve(task, ctx)** generates a `Plan` via LLM (Nvidia by default)
2. **SandboxExecutor.ExecuteWithSandbox(plan)** validates in isolated copy
3. **AgentCandidate** is built from sandbox results and LLM confidence
4. **AgentLoop.Evaluate(candidate)** runs through SafeEvolutionLoopV3
5. UCB selector is updated with the reward (delta stability if accepted, 0 if rejected)
6. Next iteration uses the UCB-selected strategy for better success rate

### UCB Arms

Default strategy arms:
- `conservative` - Low temperature, prefer minimal changes
- `balanced` - Medium temperature, balanced changes
- `aggressive` - Higher temperature, more transformative changes

New arms can be registered via `/agent/strategies` (POST).

### Evaluation Criteria

A candidate is accepted if:
- StabilityGate passes (stability delta > threshold)
- AntiCollapseDetector doesn't detect collapse (too many similar events)
- Sandbox tests pass (if configured)

Reward = `newStability - oldStability` if accepted, else `0`.

---

## 11. AgentPool — Multi-Agent UCB Competition

**File:** `internal/agent/pool.go`

`AgentPool` runs N parallel `CodeAgent` instances that compete via UCB. Each agent has its own strategy arm (default: `greedy`, `exploratory`, `conservative`). The pool selects the best result after running all agents.

### Core Types

```go
// PooledAgent is a single agent in the pool, with its own CodeAgent, sandbox,
// evolution loop, and UCB selector.
type PooledAgent struct {
    ID          string
    Strategy    string // UCB arm name
    CodeAgent   *CodeAgent
    SandboxExec *SandboxExecutor
    Loop        *evolution.AgentLoop
    LastResult  *evolution.AgentResult
    mu          sync.RWMutex
}

// AgentPool manages N parallel CodeAgents that compete via UCB.
// Each agent has its own UCB arm so the pool learns which strategy works best.
type AgentPool struct {
    agents       []*PooledAgent
    sharedLoop   *evolution.SafeEvolutionLoopV3
    sharedRewarder evolution.Rewarder
    semMemGetter func() *memory.SemanticMemory // lazily initialized shared memory
    poolMu       sync.RWMutex
}
```

### Constructor

`NewAgentPool(n, projectRoot, semMemGetter)`:
- Creates 1–8 agents (hard-capped at 8)
- Each agent gets its own `CodeAgent` (with retry LLM), `SandboxExecutor`, and `AgentLoop`
- All agents share the same `SafeEvolutionLoopV3` (shared WAL/state)
- `semMemGetter` is called lazily on first use to get the shared `SemanticMemory`

### Pipeline (per agent)

1. `CodeAgent.Solve(task, ctx)` → generates a `Plan` via LLM (Nvidia, with retry)
2. `SandboxExecutor.ExecuteWithSandbox(plan)` → validates in isolated copy
3. `runSemanticAnalysis(...)` → checks semantic risk using `SemanticMemory`
4. `AgentLoop.Evaluate(candidate, ...)` → runs through `SafeEvolutionLoopV3`, updates UCB

### Pool Selection

After `RunAll`, `BestResult()` returns the agent result with the highest score. On equal scores, prefers the result with fewer semantic reasons (simpler fix).

### UCB Integration

Each `PooledAgent` has its own `UCBSelector` arm. The shared `SafeEvolutionLoopV3` is used for evaluation, but per-agent UCB stats are tracked separately via `PoolStats()`.

---

## 12. SemanticMemory — Persistent SQLite Symbol Index

**Files:** `internal/semantic/memory/memory.go`, `schema.go`, `queries.go`

`SemanticMemory` is a SQLite-based persistent index of Go code symbols (functions, methods, types, variables, constants) and their call relationships. It enables semantic risk analysis for candidate patches.

### Database Schema

```sql
-- symbol_index: one row per symbol (func/method/type/var/const)
CREATE TABLE symbol_index (
    symbol_id  TEXT PRIMARY KEY,  -- "pkg:file_path:name" deterministic
    name       TEXT    NOT NULL,
    kind       TEXT    NOT NULL,  -- func|method|type|var|const
    package    TEXT    NOT NULL,  -- fully qualified package path
    file_path  TEXT    NOT NULL,
    line       INTEGER NOT NULL DEFAULT 0,
    exported   INTEGER NOT NULL DEFAULT 0,
    signature  TEXT    NOT NULL DEFAULT '',
    is_test    INTEGER NOT NULL DEFAULT 0
);

-- call_index: caller → callee relationships
CREATE TABLE call_index (
    caller_id TEXT NOT NULL,
    callee_id TEXT NOT NULL,
    PRIMARY KEY (caller_id, callee_id)
);

-- file_index: tracks last indexed hash for incremental re-indexing
CREATE TABLE file_index (
    file_path TEXT PRIMARY KEY,
    file_hash TEXT NOT NULL DEFAULT '',
    indexed_at INTEGER NOT NULL DEFAULT 0
);
```

### Key Design Decisions

- `symbol_id` is deterministic (`pkg:file_path:name`) so incremental re-indexing produces stable IDs
- `file_hash` (SHA-256) skips files that haven't changed since last index
- WAL journal mode + `SYNCHRONOUS=FULL` for durability
- Single connection (`SetMaxOpenConns(1)`) to avoid SQLite lock issues

### Usage in AgentPool

`runSemanticAnalysis(projectRoot, targetFile, plan, semMem)` in `pool.go` uses `SemanticMemory` to:
- Check if the patch modifies symbols with high call graph fan-out (risk propagation)
- Detect if the patch touches exported API surfaces
- Assess semantic risk score (0.0–1.0) based on call graph analysis

---

*End of corrected DESIGN.md*