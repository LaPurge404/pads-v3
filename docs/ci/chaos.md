# Chaos Engine

The Chaos Engine injects controlled failure into CI execution.

## Modes

- Silent: low probability delays
- Hard: injected failures
- Full: corruption + crashes

## Fault Types

- DelayFault
- KillWorkerFault
- CorruptWALFault

## Rule

Chaos MUST NEVER affect certification runs.
