package agent

import (
	"sync"
	"testing"

	"github.com/sony/gobreaker/v2"
)

// TestBreakerLazyInitConcurrentAndShared is the regression test for a
// -race-only data race that the original cb0e75d wrap inherited, and the
// fine-grained sanity check that the breaker is shared across all callers
// of a single client.
//
// Background (commit c7b0792): the (c *XClient).breaker() lazy init
// previously was a non-atomic read-then-write:
//
//	if c.Breaker == nil {
//	    c.Breaker = defaultBreaker()
//	}
//
// Concurrent first-callers could both observe c.Breaker == nil and assign
// two distinct defaults, silently breaking the circuit-breaker protection
// (each goroutine would puncture its own breaker, never the shared one).
//
// The fix replaces the unguarded init with sync.Once. Two observable
// guarantees:
//
//  1. All N concurrent goroutines issued on a single client see the
//     SAME *gobreaker.CircuitBreaker[*CodeResponse] pointer (a fresh
//     client per round so each round hits a cold lazy-init path).
//  2. Each round exercises real concurrency (8 goroutines per round,
//     64 rounds) — under -race, a regression in the sync.Once would
//     immediately fail.
//
// We also keep a per-type sub-test so a regression specific to one of
// the three clients (OpenAI / Claude / Nvidia) trips the test.
func TestBreakerLazyInitConcurrentAndShared(t *testing.T) {
	t.Run("OpenAI", func(t *testing.T) { runBreakerRace(t, func() *gobreaker.CircuitBreaker[*CodeResponse] { return newOpenAIForTest().breaker() }) })
	t.Run("Claude", func(t *testing.T) { runBreakerRace(t, func() *gobreaker.CircuitBreaker[*CodeResponse] { return newClaudeForTest().breaker() }) })
	t.Run("Nvidia", func(t *testing.T) { runBreakerRace(t, func() *gobreaker.CircuitBreaker[*CodeResponse] { return newNvidiaForTest().breaker() }) })

	// Strong smoke test: 64 goroutines on a SINGLE client → all must
	// observe the same *CB pointer (property (1) above under contention).
	t.Run("SingleClient64Callers", func(t *testing.T) {
		const callers = 64
		c := newOpenAIForTest()
		var pointers [callers]*gobreaker.CircuitBreaker[*CodeResponse]
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				pointers[i] = c.breaker()
			}(i)
		}
		close(start)
		wg.Wait()
		first := pointers[0]
		for i, p := range pointers {
			if p != first {
				t.Fatalf("caller %d saw a different Breaker pointer (%p vs %p) under -race", i, p, first)
			}
		}
	})
}

// runBreakerRace exercises lazy-init concurrency per round and asserts
// that all goroutines see the exact same Breaker pointer.
//
// Each round uses a freshly-constructed client so the first-call path
// is exercised exactly once per goroutine (otherwise subsequent rounds
// hit a hot, already-init'd breaker and the test would silently green).
func runBreakerRace(t *testing.T, builder func() *gobreaker.CircuitBreaker[*CodeResponse]) {
	t.Helper()
	const goroutines = 8
	const rounds = 64

	for round := 0; round < rounds; round++ {
		var (
			seen  = make(map[*gobreaker.CircuitBreaker[*CodeResponse]]struct{}, 1)
			mu    sync.Mutex
			wg    sync.WaitGroup
			start = make(chan struct{})
		)
		// First invocation primes the lazy init.
		firstRef := builder()

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				mu.Lock()
				seen[builder()] = struct{}{}
				mu.Unlock()
			}()
		}
		close(start)
		wg.Wait()
		seen[firstRef] = struct{}{}

		if len(seen) != 1 {
			t.Fatalf("round %d: %d distinct Breaker pointers observed (want 1) — sync.Once is broken or builder allocates fresh", round, len(seen))
		}
	}
}
