package evolution

import (
"net/http"
"sync"
"time"
)

type RateLimiter struct {
mu       sync.Mutex
requests map[string][]time.Time
limit    int
window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
return &RateLimiter{
requests: make(map[string][]time.Time),
limit:    limit,
window:   window,
}
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
ip := r.RemoteAddr
rl.mu.Lock()
defer rl.mu.Unlock()

now := time.Now()
var valid []time.Time
for _, t := range rl.requests[ip] {
if now.Sub(t) < rl.window {
valid = append(valid, t)
}
}
rl.requests[ip] = valid

if len(valid) >= rl.limit {
http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
return
}

rl.requests[ip] = append(rl.requests[ip], now)
next(w, r)
}
}
