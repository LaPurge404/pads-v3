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

	// Requête avec token Bearer
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer test-token-abc123")
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Deuxième requête avec le même token — doit passer (limite=2)
	w2 := httptest.NewRecorder()
	handler(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("second request should also pass, got %d", w2.Code)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := evolution.NewRateLimiter(1, time.Minute)
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer test-token-xyz")

	// Première requête OK
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request failed: got %d", w.Code)
	}

	// Deuxième requête bloquée (limite=1, déjà utilisé)
	w2 := httptest.NewRecorder()
	handler(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
}

func TestRateLimiter_RejectsWithoutToken(t *testing.T) {
	rl := evolution.NewRateLimiter(10, time.Minute)
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Requête sans token — doit retourner 401 immédiatement (pas de quota consommé)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no token, got %d", w.Code)
	}
}

func TestRateLimiter_DifferentTokensIndependent(t *testing.T) {
	rl := evolution.NewRateLimiter(1, time.Minute)
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Token A : première requête OK
	reqA := httptest.NewRequest("GET", "/", nil)
	reqA.Header.Set("Authorization", "Bearer token-A")
	wA := httptest.NewRecorder()
	handler(wA, reqA)
	if wA.Code != http.StatusOK {
		t.Fatalf("token-A first request: expected 200, got %d", wA.Code)
	}

	// Token A : deuxième requête bloquée (limite=1)
	wA2 := httptest.NewRecorder()
	handler(wA2, reqA)
	if wA2.Code != http.StatusTooManyRequests {
		t.Fatalf("token-A second request: expected 429, got %d", wA2.Code)
	}

	// Token B : première requête OK (quota indépendant)
	reqB := httptest.NewRequest("GET", "/", nil)
	reqB.Header.Set("Authorization", "Bearer token-B")
	wB := httptest.NewRecorder()
	handler(wB, reqB)
	if wB.Code != http.StatusOK {
		t.Fatalf("token-B first request: expected 200, got %d", wB.Code)
	}
}

func TestRateLimiter_InvalidBearerFormat(t *testing.T) {
	rl := evolution.NewRateLimiter(10, time.Minute)
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Format invalide (sans "Bearer ")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "just-a-token")
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid bearer format, got %d", w.Code)
	}
}