package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Orchestrator struct {
	primary  Client
	fallback Client
}

func NewOrchestrator(primary Client, fallback Client) *Orchestrator {
	return &Orchestrator{primary: primary, fallback: fallback}
}

type RunResult struct {
	Provider Provider `json:"provider"`
	Model    string   `json:"model"`
	JSON     any      `json:"json"`
}

// RunJSON tries primary first, then fallback on 429/5xx/timeouts/invalid JSON.
// It enforces "JSON only" by parsing the first valid JSON object/array found.
func (o *Orchestrator) RunJSON(ctx context.Context, req GenerateRequest, fallbackModel string, out any) (RunResult, error) {
	res, err := o.tryProvider(ctx, o.primary, req, out)
	if err == nil {
		return res, nil
	}

	// Fallback on rate limit / provider failure / bad response.
	if !(errors.Is(err, ErrRateLimited) || errors.Is(err, ErrProviderFailure) || errors.Is(err, ErrBadResponse)) {
		return RunResult{}, err
	}

	freq := req
	freq.Model = fallbackModel
	return o.tryProvider(ctx, o.fallback, freq, out)
}

func (o *Orchestrator) tryProvider(ctx context.Context, client Client, req GenerateRequest, out any) (RunResult, error) {
	// Per-provider timeout; HTTP clients use a slightly higher limit.
	pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	result, err := client.Generate(pctx, req)
	if err != nil {
		return RunResult{}, err
	}

	jsonText, ok := extractFirstJSON(result.Text)
	if !ok {
		return RunResult{}, ErrBadResponse
	}
	if err := json.Unmarshal([]byte(jsonText), out); err != nil {
		return RunResult{}, ErrBadResponse
	}
	return RunResult{Provider: result.Provider, Model: result.Model, JSON: out}, nil
}

func extractFirstJSON(s string) (string, bool) {
	// Common failure: model wraps JSON in markdown fences.
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Find first '{' or '[' and last matching '}' or ']'.
	startObj := strings.IndexByte(s, '{')
	startArr := strings.IndexByte(s, '[')
	start := -1
	if startObj >= 0 && startArr >= 0 {
		if startObj < startArr {
			start = startObj
		} else {
			start = startArr
		}
	} else if startObj >= 0 {
		start = startObj
	} else {
		start = startArr
	}
	if start < 0 {
		return "", false
	}

	endObj := strings.LastIndexByte(s, '}')
	endArr := strings.LastIndexByte(s, ']')
	end := endObj
	if endArr > end {
		end = endArr
	}
	if end <= start {
		return "", false
	}
	return strings.TrimSpace(s[start : end+1]), true
}

