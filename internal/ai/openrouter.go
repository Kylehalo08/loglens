package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenRouter is OpenAI-compatible chat completions.
// Endpoint: https://openrouter.ai/api/v1/chat/completions
type OpenRouterClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewOpenRouterClient() (*OpenRouterClient, error) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return nil, errors.New("OPENROUTER_API_KEY is required")
	}
	return &OpenRouterClient{
		apiKey:  key,
		baseURL: "https://openrouter.ai/api/v1",
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}, nil
}

func (c *OpenRouterClient) Provider() Provider { return ProviderOpenRouter }

type openaiChatReq struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Reasoning map[string]any `json:"reasoning,omitempty"`
}

type openaiChatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *OpenRouterClient) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	url := c.baseURL + "/chat/completions"

	payload := openaiChatReq{
		Model: req.Model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "user", Content: req.Prompt},
		},
		Reasoning: map[string]any{"enabled": true},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return GenerateResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return GenerateResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("%w: %v", ErrProviderFailure, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		return GenerateResult{}, ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GenerateResult{}, fmt.Errorf("%w: status %d", ErrProviderFailure, resp.StatusCode)
	}

	var parsed openaiChatResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return GenerateResult{}, fmt.Errorf("%w: invalid json", ErrBadResponse)
	}
	if len(parsed.Choices) == 0 {
		return GenerateResult{}, ErrBadResponse
	}

	return GenerateResult{
		Provider: ProviderOpenRouter,
		Model:    req.Model,
		Text:     parsed.Choices[0].Message.Content,
	}, nil
}

