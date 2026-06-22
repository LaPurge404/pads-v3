package agent

import (
	"sync"
	"testing"

	"github.com/sony/gobreaker/v2"
)

// newOpenAIForTest, newClaudeForTest and newNvidiaForTest construct a
// per-test client with a dummy API key, so breaker_race_test.go can
// exercise the (c *XClient).breaker() lazy-init path without touching
// the real OPENAI_API_KEY / ANTHROPIC_API_KEY / NVIDIA_API_KEY env vars
// or making any network call. The breaker lazy init that these helpers
// are built for is the regression target under -race — net access is
// intentionally out of scope here.
func newOpenAIForTest() *OpenAIClient {
	c := NewOpenAIClient("gpt-4o-mini")
	c.APIKey = "test-key"
	return c
}

func newClaudeForTest() *ClaudeClient {
	c := NewClaudeClient("claude-3-5-sonnet-latest")
	c.APIKey = "test-key"
	return c
}

func newNvidiaForTest() *NvidiaClient {
	c := NewNvidiaClient("meta/llama-3.1-70b-instruct")
	c.APIKey = "test-key"
	return c
}

// TestBreakerLazyInitConcurrentAndShared is the regression test for a
// -race-only data race that the new gobreaker wrap inherited, and the
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
//     SAME *gobreaker.CircuitBreaker[*CodeResponse] pointer — each
//     sub-test constructs ONE client and N goroutines call c.breaker().
//  2. Each round exercises real concurrency (8 goroutines per round,
//     64 rounds) — under -race, a regression in the sync.Once would
//     immediately fail.
//
// We also keep a per-type sub-test so a regression specific to one of
// the three clients (OpenAI / Claude / Nvidia) trips the test.
func TestBreakerLazyInitConcurrentAndShared(t *testing.T) {
	t.Run("OpenAI", func(t *testing.T) { runBreakerRace(t, newOpenAIForTest()) })
	t.Run("Claude", func(t *testing.T) { runBreakerRace(t, newClaudeForTest()) })
	t.Run("Nvidia", func(t *testing.T) { runBreakerRace(t, newNvidiaForTest()) })

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

// breakerClient is implemented by *OpenAIClient, *ClaudeClient, *NvidiaClient.
// runBreakerRace only needs c.breaker().
type breakerClient interface {
	breaker() *gobreaker.CircuitBreaker[*CodeResponse]
}

// runBreakerRace exercises lazy-init concurrency per round and asserts
// that all goroutines observe the SAME Breaker pointer on the SAME client.
//
// Each round uses the SINGLE client passed in; under sync.Once the first
// caller on any round primes the breaker and every subsequent caller on
// that round (and every future round on the same client) sees the same
// pointer — which is precisely the invariant the production code relies on.
func runBreakerRace(t *testing.T, c breakerClient) {
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

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				mu.Lock()
				seen[c.breaker()] = struct{}{}
				mu.Unlock()
			}()
		}
		close(start)
		wg.Wait()

		if len(seen) != 1 {
			t.Fatalf("round %d: %d distinct Breaker pointers observed (want 1) — sync.Once is broken", round, len(seen))
		}
	}
}
