# PADS v3 – Deterministic CI with Self-Learning Policy Engine

## Overview

PADS v3 is an event-sourced, deterministic CI system with a
self-learning policy engine. It combines DAG-based execution,
replay verification, multi-layer validation gates, chaos
engineering, and adaptive decision tuning.

## Architecture

```

Job Spec → DAG Builder → DAG Executor → Canonical Events

Gates Validation (Syntax, Semantic, Execution, Determinism)

Policy Engine (weighted scoring, hard-fail gates, chaos penalty)

Explainability (trace, replay, explanation)

Policy WAL (append-only decision log)

Learner (EMA, Z-score anomaly detection, adaptive tuning)

```

## Key Principle

> Determinism is mandatory. Chaos is optional. Certification is authoritative.
> Decisions are explainable, replayable, and continuously improving.

## Pipeline

1. **Execution** : Scheduler runs DAG, produces Canonical Events
2. **Validation** : Gates check syntax, semantics, execution, determinism
3. **Decision** : Policy Engine computes score and status (PASS/WARN/FAIL/BLOCK)
4. **Audit** : Trace and Explanation are generated and persisted to WAL
5. **Learning** : Learner replays WAL, updates EMA/Z-score, suggests tuning

## Core Components

| Package | Role |
|---------|------|
| `internal/dag` | Deterministic DAG executor |
| `internal/ci` | CI pipeline (Scheduler, Cache, Artifacts, WAL) |
| `internal/ci/gates` | Validation gates |
| `internal/ci/certification` | Replay-based determinism proof |
| `internal/ci/chaos` | Fault injection for resilience testing |
| `internal/ci/runner` | PolicyRunner: gates → policy → WAL → learner |
| `internal/policy` | Policy Engine (scoring, hard-fail, chaos penalty) |
| `internal/policy/wal` | Append-only decision log |
| `internal/policy/learner` | Adaptive tuning with EMA and Z-score anomaly detection |

## Status

- ✅ DAG execution engine
- ✅ CI gates (syntax, semantic, execution, determinism)
- ✅ Chaos engineering (silent, hard, full)
- ✅ Replay certification
- ✅ Policy engine (weighted scoring, hard-fail gates)
- ✅ Explainability (trace, replay, explanation)
- ✅ Policy WAL (append-only, replayable)
- ✅ Learner (EMA, Z-score, anomaly detection)
- ✅ Runner integration (end-to-end pipeline)
- ⚠️ Dynamic reconfiguration (TunedConfig not yet applied automatically)
- ⚠️ Learner concurrency hardening
- ⚠️ Learner state persistence (snapshot)

## Build & Test

```bash
go build ./...
go test ./...
```

License

Experimental / Research – no production warranty.
