package evolution

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// RateLimiter implements per-token (not per-IP) rate limiting.
// The key is extracted from the Authorization: Bearer *** header.
// Implements background cleanup to prevent unbounded memory growth.
type RateLimiter struct {
	mu        sync.Mutex
	requests  map[string][]time.Time
	limit     int
	window    time.Duration
	stopCh    chan struct{}
	maxTokens int // maximum number of distinct tokens to track
}

// NewRateLimiter creates a RateLimiter with the given limit per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests:  make(map[string][]time.Time),
		limit:     limit,
		window:    window,
		stopCh:    make(chan struct{}),
		maxTokens: 1000, // default max tracked tokens
	}
	go rl.cleanupLoop()
	return rl
}

// SetMaxTokens sets the maximum number of distinct tokens to track.
// Must be called before Start() if using a custom value.
func (rl *RateLimiter) SetMaxTokens(max int) {
	rl.mu.Lock()
	rl.maxTokens = max
	rl.mu.Unlock()
}

// Stop stops the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// cleanupLoop periodically removes stale entries and enforces maxTokens limit.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes stale timestamps and enforces maxTokens limit.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// First pass: filter stale entries and find oldest timestamp per token
	type tokenInfo struct {
		valid       []time.Time
		oldestValid time.Time
	}
	tokenData := make(map[string]*tokenInfo)

	for token, times := range rl.requests {
		var valid []time.Time
		for _, t := range times {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		if len(valid) > 0 {
			tokenData[token] = &tokenInfo{valid: valid, oldestValid: valid[0]}
		}
		delete(rl.requests, token)
	}

	// If over maxTokens, evict tokens with oldest activity
	if len(tokenData) > rl.maxTokens {
		// Sort by oldest valid timestamp to evict least recently active tokens
		type eviction struct {
			token  string
			oldest time.Time
		}
		evictions := make([]eviction, 0, len(tokenData))
		for token, data := range tokenData {
			evictions = append(evictions, eviction{token: token, oldest: data.oldestValid})
		}

		// Sort by oldest first (evict LRU)
		sort.Slice(evictions, func(i, j int) bool {
			return evictions[i].oldest.Before(evictions[j].oldest)
		})

		// Evict oldest tokens until we're under limit
		evictCount := len(tokenData) - rl.maxTokens
		for i := 0; i < evictCount && i < len(evictions); i++ {
			delete(tokenData, evictions[i].token)
		}
	}

	// Rebuild requests map with remaining tokens
	for token, data := range tokenData {
		rl.requests[token] = data.valid
	}

	slog.Debug("rate limiter cleanup", "tracked_tokens", len(rl.requests))
}

// extractToken returns the Bearer token from the request, or "" if absent.
func (rl *RateLimiter) extractToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// Middleware enforces the rate limit using the Bearer token as the key.
// If no token is present, it rejects immediately with 401 (does not consume quota).
// The auth check itself is done separately by authMiddleware.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := rl.extractToken(r)
		if token == "" {
			// No token → reject without consuming quota
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		rl.mu.Lock()
		defer rl.mu.Unlock()

		now := time.Now()
		var valid []time.Time
		for _, t := range rl.requests[token] {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		rl.requests[token] = valid

		if len(valid) >= rl.limit {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		rl.requests[token] = append(rl.requests[token], now)
		next(w, r)
	}
}
