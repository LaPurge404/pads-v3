# CI Gates

Gates are deterministic validators applied after execution.

## Gates List

### SyntaxGate
Validates event serialization integrity.

### SemanticGate
Ensures job structure consistency.

### ExecutionGate
Ensures WAL existence and completeness.

### DeterminismGate
Validates replay consistency of event stream.

## Rule

All gates must be pure functions:
- No side effects
- Deterministic output
