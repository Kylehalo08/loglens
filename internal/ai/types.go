package ai

import "context"

type Provider string

const (
	ProviderGemini    Provider = "gemini"
	ProviderOpenRouter Provider = "openrouter"
)

type GenerateRequest struct {
	// A short name for metrics/logging like "fr13_nlq".
	UseCase string
	// Model name for the provider.
	Model string
	// Prompt should be short and must instruct "JSON only".
	Prompt string
}

type GenerateResult struct {
	Provider Provider `json:"provider"`
	Model    string   `json:"model"`
	Text     string   `json:"text"`
}

// Client is a minimal interface so we can swap providers (SOLID: DIP).
type Client interface {
	Provider() Provider
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
}

