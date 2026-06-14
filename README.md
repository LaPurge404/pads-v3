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

+-------------------+
    |   Event Input     |
    +---------+---------+
              |
              v
    +-------------------+
    |   Storage Layer   |
    |  (SQLite + WAL)   |
    +---------+---------+
              |
              v
    +-------------------+
    | Reduction Engine  |
    | deterministic L3  |
    +---------+---------+
              |
              v
    +-------------------+
    |   Graph State     |
    | STABLE / BROKEN   |
    +-------------------+
              ^
              |
    +-------------------+
    | Fault Injection   |
    | Chaos Testing     |
    +-------------------+

---

## 🧪 Testing Strategy

PADS v3 uses **chaos-driven validation**:

### Deterministic Tests
- rebuild consistency
- duplicate event handling
- ordering stability
- replay convergence

### Chaos Tests
- WAL deletion simulation
- crash recovery
- partial writes
- randomized failure injection
- multi-run convergence validation

---

## ⚙️ Tech Stack

- Go (core engine)
- SQLite (modernc.org/sqlite driver)
- Standard library concurrency primitives
- Custom fault injection driver

---

## 📂 Project Structure

internal/ ├── chaos/        # chaos & recovery tests ├── storage/      # SQLite abstraction layer ├── reducer/      # deterministic reduction engine ├── fault/        # fault injection driver ├── compiler/     # event ingestion layer ├── resolver/     # symbol resolution ├── symbol/       # symbol system

---

## 🧬 Key Property

> The system is designed so that repeated execution over the same event log always converges to the same final graph state, even under failure conditions.

---

## 🧪 Example Guarantee

Even with:
- random IO failures
- latency spikes
- forced statement errors
- file deletion during runtime

The system still converges to:

STABLE or BROKEN (deterministic outcome)

---

## 📌 Status

✔ Core engine stable  
✔ Chaos tests passing  
✔ Fault injection validated  
✔ Recovery logic verified  

---

## 📜 License

Experimental / research project — no production guarantees.
