package agent

import (
	"context"
	"testing"
)

// TestOpenAIClientMock verifies that without an API key, the client returns a mock.
func TestOpenAIClientMock(t *testing.T) {
	// Fake key → mock mode
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
	if len(resp.Warnings) == 0 {
		t.Error("Warnings should contain mock mode notice")
	}
	// The warning must mention mock mode
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
	// No NVIDIA_API_KEY → mock mode
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
	// Verify that the client implements LLMClient
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
	// The mock fills Explanation with the prompt info
	if resp.Explanation == "" {
		t.Error("Explanation should not be empty")
	}
}
