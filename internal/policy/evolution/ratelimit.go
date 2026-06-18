package evolution

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter implements per-token (not per-IP) rate limiting.
// The key is extracted from the Authorization: Bearer <token> header.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a RateLimiter with the given limit per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
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
