# CI Architecture v0.6

## Overview

The CI system is composed of four layers:

### 1. Execution Layer
- Scheduler
- DAG Executor
- WAL event emission

Responsible for deterministic execution of CI jobs.

---

### 2. Validation Layer (Gates)
- SyntaxGate
- SemanticGate
- ExecutionGate
- DeterminismGate

Responsible for validating correctness of execution output.

---

### 3. Resilience Layer (Chaos Engine)
- Injects controlled failures
- Modes: Silent, Hard, Full
- Used for robustness testing only

---

### 4. Certification Layer
- Replay-based validation
- WAL hash comparison
- Determinism certificate generation

---

### 5. Orchestration Layer (Runner)
- Aggregates results
- Executes gates
- Produces CIReport
- Final decision engine (pre-policy engine)

---

## Data Flow

Scheduler → DAG → WAL → Certification
                       ↓
                    Chaos (optional)
                       ↓
                  Gates validation
                       ↓
                    CIReport

---

## Design Principles

- Deterministic execution by default
- Chaos is opt-in and non-production
- Certification must be reproducible
- No cyclic dependencies between layers
