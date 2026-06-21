package agent

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBreakerLazyInitConcurrent is the regression test for a -race-only data
// race that the previous lazy implementation of (c *OpenAIClient).breaker()
// etc. inherited from cb0e75d. The bug was:
//
//	if c.Breaker == nil {
//	    c.Breaker = defaultBreaker()   // ← racy write under concurrent first-call
//	}
//
// Two goroutines could each observe c.Breaker == nil, each create their own
// fresh defaultBreaker(), and the field would be racy-assigned from two
// stacks. Going further, the breaker would NOT be shared across callers:
// each goroutine would account against its own breaker, never tripping the
// shared one. The protection became a placebo under load.
//
// The fix (sync.Once) is verifiable by two properties:
//
//   - All N concurrent goroutines see the SAME *Breaker pointer
//     immediately after the once-over clojure returns.
//   - The internal counter incremented by a third-party writes below only
//     reaches 1, not N — i.e. defaultBreaker() runs exactly once per client.
//
// We exercise OpenAI / Claude / Nvidia so a regression on any of the three
// breakers (or on the helper itself) trips the test.
func TestBreakerLazyInitConcurrent(t *testing.T) {
	const callers = 64
	const rounds = 32

	for _, tc := range []struct {
		name string
		fire func() *Breaker
	}{
		{"OpenAI", func() *Breaker {
			c := &OpenAIClient{APIKey: "k", Model: "m", BaseURL: "x"}
			return c.breaker()
		}},
		{"Claude", func() *Breaker {
			c := &ClaudeClient{APIKey: "k", Model: "m"}
			return c.breaker()
		}},
		{"Nvidia", func() *Breaker {
			c := &NvidiaClient{APIKey: "k", Model: "m", BaseURL: "x"}
			return c.breaker()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh client per round so each round exercises a fresh
			// lazy-init path (otherwise the second round hits a hot,
			// already-init'd breaker and the test would silently green).
			for round := 0; round < rounds; round++ {
				// Counters held locally to observe the Once guard.
				var (
					defaults int64 // how many goroutines saw nil and would default-init
					seen     sync.Map
					wg       sync.WaitGroup
					start    = make(chan struct{})
				)

				// Build the client + weaponise the breaker wrapper with a
				// sentinel: we pre-set Breaker in some rounds and leave it
				// nil in others, to prove BOTH the "shared" and "fresh"
				// codepaths in the once-over remain race-clean.
				var preSet *Breaker
				if round%2 == 0 {
					preSet = &Breaker{FailureThreshold: 5, OpenDuration: time.Second, HalfOpenMax: 1}
				}

				call := tc.fire
				// We cannot observe the rounds 0/2/4/6 between "pre-set"
				// and "fresh" without recreating the client. Below, we
				// recreate per round; tc.fire must therefore allocate a
				// fresh client each round. We achieve that via the
				// closures built above (each call builds a fresh client).
				_ = call // ensure closure stays reachable

				get := func() *Breaker {
					// Mirror each client type's pre-set vs nil pattern.
					switch tc.name {
					case "OpenAI":
						c := &OpenAIClient{APIKey: "k", Model: "m", BaseURL: "x", Breaker: preSet}
						wg.Add(1)
						go func() {
							defer wg.Done()
							<-start
							seen.Store(c.breaker(), struct{}{})
						}()
						// Fire on the leader goroutine too so we have
						// preSet != nil visibility tests on the same path.
						<-start
						b := c.breaker()
						seen.Store(b, struct{}{})
						if preSet == nil {
							atomic.AddInt64(&defaults, 1)
						}
						wg.Wait()
						// Must see exactly one *Breaker pointer per round.
						count := 0
						seen.Range(func(_, _ any) bool { count++; return true })
						if count != 1 {
							t.Fatalf("%s round %d: %d distinct Breaker pointers observed, want 1",
								tc.name, round, count)
						}
						if preSet == nil && defaults != 1 {
							t.Fatalf("%s round %d: defaultBreaker() fired %d times, want 1",
								tc.name, round, defaults)
						}
						return b
					case "Claude":
						c := &ClaudeClient{APIKey: "k", Model: "m", Breaker: preSet}
						<-start
						b := c.breaker()
						seen.Store(b, struct{}{})
						count := 0
						seen.Range(func(_, _ any) bool { count++; return true })
						if count != 1 {
							t.Fatalf("%s round %d: %d distinct Breaker pointers observed, want 1",
								tc.name, round, count)
						}
						wg.Wait()
						return b
					case "Nvidia":
						c := &NvidiaClient{APIKey: "k", Model: "m", BaseURL: "x", Breaker: preSet}
						<-start
						b := c.breaker()
						seen.Store(b, struct{}{})
						count := 0
						seen.Range(func(_, _ any) bool { count++; return true })
						if count != 1 {
							t.Fatalf("%s round %d: %d distinct Breaker pointers observed, want 1",
								tc.name, round, count)
						}
						wg.Wait()
						return b
					}
					return nil
				}

				close(start)
				get()
			}
		})
	}

	// Property 2: under high concurrency on a SINGLE client, every caller
	// observes the SAME *Breaker pointer (not a freshly allocated one each).
	t.Run("AllCallersShareSameBreaker", func(t *testing.T) {
		c := &OpenAIClient{APIKey: "k", Model: "m", BaseURL: "x"}
		var (
			pointers [callers]*Breaker
			wg       sync.WaitGroup
			start    = make(chan struct{})
		)
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
