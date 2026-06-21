// Package agent provides a small in-process circuit breaker for LLM clients.
//
// We deliberately avoid an external dependency here: the project keeps go.mod
// minimal (only modernc.org/sqlite) and a 70-line breaker covers the exact
// state machine we need (Closed → Open → Half-Open). When the load profile
// outgrows this implementation, swap for sony/gobreaker behind the same
// LLMClient interface — callers won't notice.
package agent

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by Breaker.Allow when the breaker is open and
// the request should be short-circuited.
var ErrCircuitOpen = errors.New("circuit breaker open")

// Breaker is a closed/open/half-open circuit breaker. After
// FailureThreshold consecutive failures (any error returned by f) it opens for
// OpenDuration. The first call after that window becomes a half-open probe:
// success closes the breaker, failure re-opens it. Thread-safe.
type Breaker struct {
	FailureThreshold int
	OpenDuration     time.Duration
	HalfOpenMax      int // max concurrent half-open probes; default 1

	mu               sync.Mutex
	state            int // 0 closed, 1 open, 2 half-open
	consecutiveFails int
	openedAt         time.Time
	halfOpenInFlight int
}

// state constants — kept local to avoid leaking implementation.
const (
	breakerClosed   = 0
	breakerOpen     = 1
	breakerHalfOpen = 2
)

// Do wraps f in the breaker. ok=false with ErrCircuitOpen means the request
// should not be attempted. ok=true means f was invoked and the (err) pair is
// the result to use.
func (b *Breaker) Do(f func() error) error {
	halfOpenMax := b.HalfOpenMax
	if halfOpenMax <= 0 {
		halfOpenMax = 1
	}
	failureThreshold := b.FailureThreshold
	if failureThreshold <= 0 {
		failureThreshold = 5
	}

	// Decide whether to allow the call.
	allowed, isHalfOpen := false, false
	b.mu.Lock()
	switch b.state {
	case breakerClosed:
		allowed = true
	case breakerOpen:
		if time.Since(b.openedAt) >= b.OpenDuration {
			b.state = breakerHalfOpen
			b.halfOpenInFlight = 1
			allowed = true
			isHalfOpen = true
		}
	case breakerHalfOpen:
		if b.halfOpenInFlight < halfOpenMax {
			b.halfOpenInFlight++
			allowed = true
			isHalfOpen = true
		}
	}
	b.mu.Unlock()

	if !allowed {
		return ErrCircuitOpen
	}

	err := f()

	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case isHalfOpen:
		b.halfOpenInFlight--
		if err == nil {
			b.state = breakerClosed
			b.consecutiveFails = 0
		} else {
			b.state = breakerOpen
			b.openedAt = time.Now()
		}
	default:
		if err == nil {
			b.consecutiveFails = 0
			return err
		}
		b.consecutiveFails++
		if b.consecutiveFails >= failureThreshold {
			b.state = breakerOpen
			b.openedAt = time.Now()
		}
	}
	return err
}

// State returns the current breaker state for observability / health checks.
// Returns one of "closed", "open", "half-open".
func (b *Breaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case breakerOpen:
		return "open"
	case breakerHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}
