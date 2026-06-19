// Package agent provides the PADS autonomous coding agent.
package agent

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements the circuit breaker pattern per LLM client.
// It tracks consecutive failures; after maxFailures the circuit opens
// for recoveryWindow, then transitions to half-open to test recovery.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            circuitState // closed | open | halfOpen
	failures         int          // consecutive failures in closed state
	successes        int          // consecutive successes in half-open state needed to close
	lastFailure      time.Time
	recoveryWindow   time.Duration
	maxFailures      int
	halfOpenSuccesses int         // successes needed in half-open to close circuit
}

// circuitState represents the current state of the circuit breaker.
type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
func NewCircuitBreaker(maxFailures int, recoveryWindow time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:       maxFailures,
		recoveryWindow:    recoveryWindow,
		halfOpenSuccesses: 2, // need 2 successes in half-open to fully close
		state:             stateClosed,
	}
}

// State returns the current state of the circuit as a string for observability.
func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// Allow checks whether a request should proceed. If the circuit is open,
// it checks whether the recovery window has elapsed and transitions to
// half-open if so. Returns nil if the request is allowed, ErrCircuitOpen otherwise.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateOpen:
		if time.Since(cb.lastFailure) >= cb.recoveryWindow {
			cb.state = stateHalfOpen
			cb.successes = 0
			return nil // allow one test request
		}
		return ErrCircuitOpen
	case stateHalfOpen:
		return nil // allow test requests through
	default:
		return nil
	}
}

// RecordSuccess records a successful call. In half-open state, increments
// the success counter and transitions to closed after halfOpenSuccesses.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		cb.failures = 0 // reset on success

	case stateHalfOpen:
		cb.successes++
		if cb.successes >= cb.halfOpenSuccesses {
			cb.state = stateClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// RecordFailure records a failed call. In closed state, increments the
// failure counter and opens the circuit if maxFailures is reached.
// In half-open state, immediately re-opens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.failures >= cb.maxFailures {
			cb.state = stateOpen
		}

	case stateHalfOpen:
		// Any failure in half-open re-opens the circuit.
		cb.state = stateOpen
		cb.lastFailure = time.Now()
		cb.successes = 0
	}
}

// Call executes fn through the circuit breaker. It first checks Allow(),
// then executes fn, then records success or failure based on the result.
// If the circuit is open, returns ErrCircuitOpen immediately without calling fn.
func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	if err := cb.Allow(); err != nil {
		return err
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}
	cb.RecordSuccess()
	return nil
}