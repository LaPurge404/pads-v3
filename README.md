🧠 PADS v3 — Deterministic Execution & Chaos-Resilient Debugging System

PADS v3 is an experimental Go-based system designed to validate deterministic state reconstruction under failure-prone environments using event replay, reduction loops, and chaos injection testing.

The core idea:

> The system must always converge to the same final state, even under corruption, crashes, or randomized failures.




---

🎯 Core Objectives

PADS v3 validates that a system can:

Reconstruct state deterministically from event logs

Maintain consistency of L3 graph state across rebuilds

Survive WAL corruption and file loss

Handle randomized execution and IO failures

Converge to a stable final state under repeated reduction loops



---

🏗️ System Architecture

The system is composed of four main layers:

1. Event Layer

Stores raw execution events and relationships.

events → execution results

event_nodes → mapping between events and nodes



---

2. Storage Layer (SQLite)

Persistence engine responsible for:

Schema management

Transactional consistency

Graph state storage (graph_state)


Backend: modernc.org/sqlite


---

3. Reduction Engine (Core Logic)

Deterministic state reconstruction system.

Responsibilities:

Replay event logs

Build graph_state

Ensure convergence to:

STABLE

BROKEN



This layer is purely deterministic.


---

4. Fault Injection Layer (Chaos Engine)

Simulates real-world system failures:

Latency injection

IO errors

Write failures

SQLITE_BUSY conditions

Random execution faults


Used exclusively for robustness validation.


---

🔁 System Flow

┌────────────────────────────┐
            │     Event Input Layer     │
            │  (execution + metadata)   │
            └─────────────┬──────────────┘
                          │
                          ▼
            ┌────────────────────────────┐
            │      SQLite Storage       │
            │   (WAL + persistence)     │
            └─────────────┬──────────────┘
                          │
                          ▼
            ┌────────────────────────────┐
            │   Reduction Engine (L3)   │
            │   deterministic replay    │
            └─────────────┬──────────────┘
                          │
                          ▼
            ┌────────────────────────────┐
            │      Graph State L3       │
            │   STABLE / BROKEN         │
            └─────────────┬──────────────┘
                          ▲
                          │
            ┌────────────────────────────┐
            │   Fault Injection Layer    │
            │   (Chaos Testing Engine)   │
            └────────────────────────────┘


---

🧪 Testing Strategy

PADS v3 uses chaos-driven validation.

Deterministic Tests

Full rebuild from scratch consistency

Duplicate event handling

Event ordering stability

Replay convergence validation


Chaos Tests

WAL file deletion simulation

Crash recovery validation

Partial writes and corrupted state

Random IO failure injection

Multi-run convergence validation



---

⚙️ Tech Stack

Go (core engine)

SQLite (modernc.org/sqlite)

Standard library concurrency primitives

Custom fault injection driver



---

📁 Project Structure

internal/
├── chaos/       → chaos & recovery tests
├── storage/     → SQLite abstraction layer
├── reducer/     → deterministic reduction engine
├── fault/       → fault injection driver
├── resolver/    → event resolution layer
├── symbol/      → symbolic utilities


---

🧬 Key Property

The system guarantees:

> Repeated execution over the same event log always converges to the same final graph state.



Even under:

IO failures

Latency spikes

Forced execution errors

File deletion during runtime



---

🧪 Convergence Guarantee

Despite chaos conditions, the system always converges to:

STABLE or BROKEN

This is the core invariant of PADS v3.


---

📌 Status

✔ Core reduction engine stable

✔ Chaos testing validated

✔ Fault injection operational

✔ Recovery mechanisms verified



---

📜 License

Experimental / research system — no production guarantees.
