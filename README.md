# PADS v3 — CI Architecture v0.6

## Overview

This project implements a deterministic CI system with:

- DAG-based execution engine
- Replay-based certification
- Multi-layer validation gates
- Chaos engineering subsystem
- Orchestration reporting layer

---

## Architecture

See `docs/ci/architecture_v0.6.md`

---

## Core Components

### Execution
- Scheduler
- DAG Executor
- WAL event stream

### Validation (Gates)
- SyntaxGate
- SemanticGate
- ExecutionGate
- DeterminismGate

### Chaos Engine
- Fault injection system
- Controlled failure testing

### Certification
- Replay verification
- Determinism proof via hash comparison

### Runner
- CI aggregation layer
- Produces CIReport

---

## Key Principle

> Determinism is mandatory. Chaos is optional. Certification is authoritative.

---

## Build

```bash
go build ./...
go test ./...
```

---

Status

v0.6 stable – ready for Policy Engine v0.7
