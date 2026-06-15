package evolution_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "pads-v3/internal/policy/evolution"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
    rl := evolution.NewRateLimiter(2, time.Minute)
    handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    handler(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", w.Code)
    }

    w2 := httptest.NewRecorder()
    handler(w2, req)
    if w2.Code != http.StatusOK {
        t.Fatalf("second request should also pass")
    }
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
    rl := evolution.NewRateLimiter(1, time.Minute)
    handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    req := httptest.NewRequest("GET", "/", nil)
    // Première requête OK
    w := httptest.NewRecorder()
    handler(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("first request failed")
    }

    // Deuxième requête bloquée
    w2 := httptest.NewRecorder()
    handler(w2, req)
    if w2.Code != http.StatusTooManyRequests {
        t.Fatalf("expected 429, got %d", w2.Code)
    }
}
