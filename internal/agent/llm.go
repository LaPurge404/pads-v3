package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// LLMClient is the interface for LLM providers (OpenAI, Claude, etc.).
type LLMClient interface {
	// GenerateCode asks the LLM to generate code for a given task context.
	GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error)
}

// CodePrompt describes a code generation request.
type CodePrompt struct {
	// Task description in natural language
	Task string
	// The file path where the code should be applied
	FilePath string
	// Language (go, python, etc.)
	Language string
	// Additional context from the codebase
	Context string
	// Constraints or requirements
	Constraints string
	// Temperature for LLM sampling (0.0-2.0, default 0.3 if 0).
	// Set by ApplyStrategyToLLM based on UCB strategy.
	Temperature float64
}

// CodeResponse contains the LLM's response for code generation.
type CodeResponse struct {
	// The generated code patch
	Patch string
	// Explanation of what was changed
	Explanation string
	// Confidence score (0-1) in the suggestion
	Confidence float64
	// Any warnings or concerns
	Warnings []string
}

// OpenAIClient implements LLMClient using the OpenAI API.
type OpenAIClient struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// NewOpenAIClient creates an OpenAI LLM client.
// API key is read from OPENAI_API_KEY environment variable.
func NewOpenAIClient(model string) *OpenAIClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-development"
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		Timeout: 60 * time.Second,
	}
}

// GenerateCode implements LLMClient.
func (c *OpenAIClient) GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error) {
	if c.APIKey == "dummy-key-for-development" {
		return c.mockResponse(prompt)
	}

	systemPrompt := `You are PADS, an autonomous code improvement agent.
Your task is to generate code fixes or improvements for the given file.
Respond with a JSON object containing:
- "patch": the code changes to apply (as a full file replacement or diff)
- "explanation": brief explanation of what changed
- "confidence": your confidence level (0.0-1.0)
- "warnings": any concerns or caveats`

	userPrompt := fmt.Sprintf(`## Task
%s

## Target File
%s

## Language
%s

## Context
%s

## Constraints
%s

Generate the code change:`, prompt.Task, prompt.FilePath, prompt.Language, prompt.Context, prompt.Constraints)

	reqBody := map[string]interface{}{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": temperatureFor(prompt.Temperature),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("OpenAI LLM retry", "attempt", attempt+1, "maxRetries", maxRetries, "backoff", backoff, "err", lastErr)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		client := &http.Client{Timeout: c.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send request: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: status %d", resp.StatusCode)
			resp.Body.Close()
			if resp.StatusCode < 500 && resp.StatusCode != 429 {
				break
			}
			continue
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			lastErr = fmt.Errorf("decode response: %w", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if len(result.Choices) == 0 {
			lastErr = fmt.Errorf("no response from LLM")
			continue
		}

		return c.parseResponse(result.Choices[0].Message.Content, prompt)
	}

	slog.Error("OpenAI LLM exhausted retries", "attempts", maxRetries, "lastErr", lastErr)
	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

// parseResponse parses the LLM's text response into a CodeResponse.
func (c *OpenAIClient) parseResponse(content string, prompt CodePrompt) (*CodeResponse, error) {
	// Try to extract JSON from the response
	var codeResp CodeResponse

	// First, try to parse the entire content as JSON
	if err := json.Unmarshal([]byte(content), &codeResp); err != nil {
		// If that fails, treat the content as the patch directly
		codeResp.Patch = content
		codeResp.Explanation = "Generated by LLM"
		codeResp.Confidence = 0.7
	}

	return &codeResp, nil
}

// mockResponse returns a mock response for development without API keys.
func (c *OpenAIClient) mockResponse(prompt CodePrompt) (*CodeResponse, error) {
	return &CodeResponse{
		Patch:       "// Mock: set OPENAI_API_KEY to enable real LLM calls",
		Explanation: fmt.Sprintf("Mock response for task: %s on %s", prompt.Task, prompt.FilePath),
		Confidence:  0.5,
		Warnings:    []string{"Running in mock mode - set OPENAI_API_KEY"},
	}, nil
}

// ClaudeClient implements LLMClient using Anthropic's Claude API.
type ClaudeClient struct {
	APIKey  string
	Model   string
	Timeout time.Duration
}

// NewClaudeClient creates a Claude LLM client.
func NewClaudeClient(model string) *ClaudeClient {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = "dummy-key-for-development"
	}
	return &ClaudeClient{
		APIKey:  apiKey,
		Model:   model,
		Timeout: 60 * time.Second,
	}
}

// GenerateCode implements LLMClient (placeholder for Claude).
func (c *ClaudeClient) GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error) {
	if c.APIKey == "dummy-key-for-development" {
		return &CodeResponse{
			Patch:       "// Mock: set ANTHROPIC_API_KEY to enable real LLM calls",
			Explanation: fmt.Sprintf("Mock response for task: %s", prompt.Task),
			Confidence:  0.5,
			Warnings:    []string{"Running in mock mode"},
		}, nil
	}
	return nil, fmt.Errorf("Claude API not yet implemented")
}

// Retry constants for LLM calls.
const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
	maxBackoff     = 8 * time.Second
)

// temperatureFor returns temp if it is a positive value, otherwise the default 0.3.
// This lets callers leave Temperature at 0 to mean "use default".
func temperatureFor(temp float64) float64 {
	if temp > 0 {
		return temp
	}
	return 0.3
}

// NvidiaClient implements LLMClient using the NVIDIA API (NIM/inference endpoints).
// This is the DEFAULT client when no specific provider is requested.
type NvidiaClient struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// NewNvidiaClient creates a NVIDIA LLM client.
// API key is read from NVIDIA_API_KEY environment variable.
// This is the default LLM client when no provider is specified.
func NewNvidiaClient(model string) *NvidiaClient {
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		// Fallback to common NVIDIA API endpoints that accept API keys
		apiKey = "dummy-key-for-development"
	}
	baseURL := os.Getenv("NVIDIA_BASE_URL")
	if baseURL == "" {
		// Default NVIDIA NIM endpoint
		baseURL = "https://integrate.api.nvidia.com/v1"
	}
	return &NvidiaClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		Timeout: 120 * time.Second,
	}
}

// GenerateCode implements LLMClient using NVIDIA's completion API.
func (c *NvidiaClient) GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error) {
	if c.APIKey == "dummy-key-for-development" {
		return c.mockResponse(prompt)
	}

	systemPrompt := `You are PADS, an autonomous code improvement agent.
Your task is to generate code fixes or improvements for the given file.
Respond with a JSON object containing:
- "patch": the code changes to apply (as a full file replacement or diff)
- "explanation": brief explanation of what changed
- "confidence": your confidence level (0.0-1.0)
- "warnings": any concerns or caveats`

	userPrompt := fmt.Sprintf(`## Task
%s

## Target File
%s

## Language
%s

## Context
%s

## Constraints
%s

Generate the code change:`, prompt.Task, prompt.FilePath, prompt.Language, prompt.Context, prompt.Constraints)

	reqBody := map[string]interface{}{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": temperatureFor(prompt.Temperature),
		"max_tokens":  4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("NVIDIA LLM retry", "attempt", attempt+1, "maxRetries", maxRetries, "backoff", backoff, "err", lastErr)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		client := &http.Client{Timeout: c.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send request: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("NVIDIA API error: status %d", resp.StatusCode)
			resp.Body.Close()
			// Don't retry on client errors (4xx except 429)
			if resp.StatusCode < 500 && resp.StatusCode != 429 {
				break
			}
			continue
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			lastErr = fmt.Errorf("decode response: %w", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if len(result.Choices) == 0 {
			lastErr = fmt.Errorf("no response from NVIDIA LLM")
			continue
		}

		return c.parseResponse(result.Choices[0].Message.Content, prompt)
	}

	slog.Error("NVIDIA LLM exhausted retries", "attempts", maxRetries, "lastErr", lastErr)
	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}

// parseResponse parses the LLM's text response into a CodeResponse.
func (c *NvidiaClient) parseResponse(content string, prompt CodePrompt) (*CodeResponse, error) {
	var codeResp CodeResponse
	if err := json.Unmarshal([]byte(content), &codeResp); err != nil {
		codeResp.Patch = content
		codeResp.Explanation = "Generated by NVIDIA LLM"
		codeResp.Confidence = 0.7
	}
	return &codeResp, nil
}

// mockResponse returns a mock response for development without API keys.
func (c *NvidiaClient) mockResponse(prompt CodePrompt) (*CodeResponse, error) {
	return &CodeResponse{
		Patch:       "// Mock: set NVIDIA_API_KEY to enable real LLM calls",
		Explanation: fmt.Sprintf("Mock response for task: %s on %s", prompt.Task, prompt.FilePath),
		Confidence:  0.5,
		Warnings:    []string{"Running in mock mode - set NVIDIA_API_KEY"},
	}, nil
}

// NewDefaultLLMClient creates the default LLM client (Nvidia).
// This should be used when no specific provider is requested.
func NewDefaultLLMClient() LLMClient {
	// Default to Nvidia - read model from environment or use a default
	model := os.Getenv("NVIDIA_MODEL")
	if model == "" {
		model = "meta/llama-3.1-70b-instruct"
	}
	return NewNvidiaClient(model)
}
