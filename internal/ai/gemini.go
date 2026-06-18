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

// Gemini client for Google AI Studio.
// Docs: https://ai.google.dev/
type GeminiClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewGeminiClient() (*GeminiClient, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, errors.New("GEMINI_API_KEY is required")
	}
	return &GeminiClient{
		apiKey:  key,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		httpClient: &http.Client{
			Timeout: 25 * time.Second,
		},
	}, nil
}

func (c *GeminiClient) Provider() Provider { return ProviderGemini }

type geminiReq struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (c *GeminiClient) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, req.Model)

	payload := geminiReq{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: req.Prompt},
				},
			},
		},
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
	httpReq.Header.Set("X-goog-api-key", c.apiKey)

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

	var parsed geminiResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return GenerateResult{}, fmt.Errorf("%w: invalid json", ErrBadResponse)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return GenerateResult{}, ErrBadResponse
	}

	return GenerateResult{
		Provider: ProviderGemini,
		Model:    req.Model,
		Text:     parsed.Candidates[0].Content.Parts[0].Text,
	}, nil
}

