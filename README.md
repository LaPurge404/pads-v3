# PADS v3 — Predictive Analysis & Debugging System

PADS v3 is a Go-based experimental system designed to build a resilient execution and analysis pipeline with deterministic recovery, chaos testing, and fault-injection validation.

It focuses on **system robustness under failure conditions**, including:
- database corruption
- execution crashes
- partial event loss
- randomized fault injection

---

## 🚀 Core Goals

PADS v3 aims to validate that a system can:

- Recover deterministically from inconsistent states
- Maintain L3 graph consistency across rebuilds
- Handle WAL corruption and file loss
- Survive randomized runtime failures
- Converge to a stable state under repeated reduction loops

---

## 🧠 Architecture Overview

The system is composed of four main layers:

### 1. Event Layer
Stores raw execution events and their metadata.

- `events` → execution results
- `event_nodes` → mapping between events and nodes

### 2. Storage Layer
Persistent SQLite-based state engine.

- schema management
- transactional consistency
- graph state storage

### 3. Reduction Engine
Deterministic state reconstruction system.

- processes event logs
- builds `graph_state`
- ensures convergence to STABLE or BROKEN states

### 4. Fault Injection Layer
Chaos testing system that simulates real-world failures:

- latency injection
- IO errors
- write failures
- SQLITE_BUSY conditions
- random execution faults

---

## 🔁 System Flow
