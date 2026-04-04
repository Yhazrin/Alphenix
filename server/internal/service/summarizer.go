package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const summarizerSystemPrompt = `You are a conversation summarizer. Given a list of conversation messages, produce a concise summary that preserves:
1. The overall objective and current progress
2. Key decisions made
3. Important results or outputs from tool calls
4. Any blockers or unresolved issues

Keep the summary under 500 words. Use clear, structured text. Do not include pleasantries or meta-commentary.`

// NewLLMSummarizer creates a Summarizer function that calls the Anthropic API.
// Returns nil if ANTHROPIC_API_KEY is not set.
func NewLLMSummarizer() func(ctx context.Context, messages []Message) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil
	}

	model := os.Getenv("SUMMARIZER_MODEL")
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	return func(ctx context.Context, messages []Message) (string, error) {
		return callAnthropicSummarize(ctx, apiKey, model, messages)
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func callAnthropicSummarize(ctx context.Context, apiKey, model string, messages []Message) (string, error) {
	// Build conversation text for summarization
	var parts []string
	for _, msg := range messages {
		preview := msg.Content
		if len(preview) > 500 {
			preview = preview[:500] + "... [truncated]"
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", msg.Role, preview))
	}
	conversationText := strings.Join(parts, "\n")

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: 1024,
		System:    summarizerSystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: conversationText},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic API call: %w", err)
	}
	defer resp.Body.Close()

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Error != nil {
		slog.Warn("llm_summarizer: Anthropic API error", "error", apiResp.Error.Message)
		return "", fmt.Errorf("anthropic API error: %s", apiResp.Error.Message)
	}

	for _, block := range apiResp.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("empty response from Anthropic API")
}
