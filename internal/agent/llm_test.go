package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestOpenAIClientMock verifies that without an API key, the client returns a mock.
func TestOpenAIClientMock(t *testing.T) {
	client := NewOpenAIClient("gpt-4o-mini")
	resp, err := client.GenerateCode(context.Background(), CodePrompt{
		Task:     "Add a logging statement",
		FilePath: "server.go",
		Language: "go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Patch == "" {
		t.Error("Patch should not be empty in mock response")
	}
	if resp.Confidence == 0 {
		t.Error("Confidence should not be zero in mock response")
	}
	found := false
	for _, w := range resp.Warnings {
		if w == "Running in mock mode - set OPENAI_API_KEY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mock warning, got: %v", resp.Warnings)
	}
}

// TestNvidiaClientMock verifies that without NVIDIA_API_KEY,
// the Nvidia client returns a mock.
func TestNvidiaClientMock(t *testing.T) {
	client := NewNvidiaClient("meta/llama-3.1-70b-instruct")
	resp, err := client.GenerateCode(context.Background(), CodePrompt{
		Task:     "Fix nil pointer dereference",
		FilePath: "handler.go",
		Language: "go",
		Context:  "func init() {}",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Patch == "" {
		t.Error("Patch should not be empty in mock response")
	}
	if resp.Confidence == 0 {
		t.Error("Confidence should not be zero")
	}
	found := false
	for _, w := range resp.Warnings {
		if w == "Running in mock mode - set NVIDIA_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mock warning, got: %v", resp.Warnings)
	}
}

// TestNewDefaultLLMClient verifies that the default client is created without error.
func TestNewDefaultLLMClient(t *testing.T) {
	client := NewDefaultLLMClient()
	if client == nil {
		t.Fatal("NewDefaultLLMClient returned nil")
	}
	var _ LLMClient = client
}

// TestCodePromptFields verifies that CodePrompt fields are used correctly.
func TestCodePromptFields(t *testing.T) {
	client := NewOpenAIClient("gpt-4o-mini")
	prompt := CodePrompt{
		Task:        "Refactor error handling",
		FilePath:    "api.go",
		Language:    "go",
		Context:     "// existing code",
		Constraints: "Must be idiomatic Go",
	}
	resp, err := client.GenerateCode(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Explanation == "" {
		t.Error("Explanation should not be empty")
	}
}

// TestLLMRetryOnRateLimit verifies that the LLM client retries on HTTP 429
// and succeeds on the third attempt.
func TestLLMRetryOnRateLimit(t *testing.T) {
	var hitCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitCount++
		count := hitCount
		mu.Unlock()

		if count <= 2 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"content": `{"patch":"// ok","explanation":"rate limit passed","confidence":0.9,"warnings":[]}`,
				}},
			},
		})
	}))
	defer server.Close()

	client := NewNvidiaClient("meta/llama-3.1-70b-instruct")
	client.APIKey = "test-key"
	client.BaseURL = server.URL + "/v1"
	client.Timeout = 5 * time.Second

	resp, err := client.GenerateCode(context.Background(), CodePrompt{
		Task:     "rate limit test",
		FilePath: "test.go",
		Language: "go",
	})

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if resp == nil || resp.Patch == "" {
		t.Fatal("expected non-empty patch after successful retry")
	}

	mu.Lock()
	hits := hitCount
	mu.Unlock()
	if hits != 3 {
		t.Errorf("expected exactly 3 hits, got %d", hits)
	}
}

// TestLLMRetryOn500 verifies retry on 500 and eventual success.
func TestLLMRetryOn500(t *testing.T) {
	var hitCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitCount++
		count := hitCount
		mu.Unlock()

		if count == 1 {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"content": `{"patch":"// ok","explanation":"ok","confidence":0.9,"warnings":[]}`,
				}},
			},
		})
	}))
	defer server.Close()

	client := NewNvidiaClient("meta/llama-3.1-70b-instruct")
	client.APIKey = "test-key"
	client.BaseURL = server.URL + "/v1"
	client.Timeout = 5 * time.Second

	resp, err := client.GenerateCode(context.Background(), CodePrompt{
		Task:     "500 retry test",
		FilePath: "test.go",
		Language: "go",
	})

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if resp == nil || resp.Patch == "" {
		t.Fatal("expected non-empty patch")
	}

	mu.Lock()
	hits := hitCount
	mu.Unlock()
	if hits != 2 {
		t.Errorf("expected 2 hits (1 x 500 + 1 x 200), got %d", hits)
	}
}

// TestLLMNoRetryOn400 verifies that 400 is NOT retried.
func TestLLMNoRetryOn400(t *testing.T) {
	var hitCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hitCount++
		mu.Unlock()
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewNvidiaClient("meta/llama-3.1-70b-instruct")
	client.APIKey = "test-key"
	client.BaseURL = server.URL + "/v1"
	client.Timeout = 5 * time.Second

	_, err := client.GenerateCode(context.Background(), CodePrompt{
		Task:     "bad request test",
		FilePath: "test.go",
		Language: "go",
	})

	if err == nil {
		t.Fatal("expected error on 400")
	}

	mu.Lock()
	hits := hitCount
	mu.Unlock()
	if hits != 1 {
		t.Errorf("expected exactly 1 hit (no retry on 400), got %d", hits)
	}
}

// TestLLMRetryExhausted verifies that after max retries are exhausted,
// the client returns an error mentioning the retry count.
func TestLLMRetryExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewNvidiaClient("meta/llama-3.1-70b-instruct")
	client.APIKey = "test-key"
	client.BaseURL = server.URL + "/v1"
	client.Timeout = 5 * time.Second

	_, err := client.GenerateCode(context.Background(), CodePrompt{
		Task:     "always fails",
		FilePath: "test.go",
		Language: "go",
	})

	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if !strings.Contains(err.Error(), "attempts") && !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected error mentioning attempts, got: %v", err)
	}
}

// TestLLMSubprocessRetry and TestLLMSubprocessTimeout are reserved for
// manual verification using a real subprocess server (avoids in-process SIGPIPE
// state corruption from timing-dependent tests). They require the mock server
// at testdata/mock_llm_server/main.go and are skipped in normal runs.
// Run manually with: go test -v -run "TestLLMSubprocess" ./internal/agent