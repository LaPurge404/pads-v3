package agent

import (
	"context"
	"testing"
)

// TestOpenAIClientMock vérifie que sans clef API, le client retourne un mock.
func TestOpenAIClientMock(t *testing.T) {
	// Clef bidon → mode mock
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
	// Le warning doit mentionner le mode mock
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

// TestNvidiaClientMock vérifie que sans clef NVIDIA_API_KEY,
// le client Nvidia retourne un mock.
func TestNvidiaClientMock(t *testing.T) {
	// Pas de NVIDIA_API_KEY → mode mock
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

// TestNewDefaultLLMClient vérifie que le client par défaut est créé sans erreur.
func TestNewDefaultLLMClient(t *testing.T) {
	client := NewDefaultLLMClient()
	if client == nil {
		t.Fatal("NewDefaultLLMClient returned nil")
	}
	// Vérifier que le client implémente bien LLMClient
	var _ LLMClient = client
}

// TestCodePromptFields vérifie que les champs de CodePrompt sont bien utilisés.
func TestCodePromptFields(t *testing.T) {
	client := NewOpenAIClient("gpt-4o-mini")
	prompt := CodePrompt{
		Task:       "Refactor error handling",
		FilePath:   "api.go",
		Language:   "go",
		Context:    "// existing code",
		Constraints: "Must be idiomatic Go",
	}
	resp, err := client.GenerateCode(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Le mock remplit Explanation avec les infos du prompt
	if resp.Explanation == "" {
		t.Error("Explanation should not be empty")
	}
}