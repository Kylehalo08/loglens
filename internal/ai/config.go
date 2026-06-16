package ai

import "os"

// OpenRouterFallbackModel is used when Gemini returns 429/5xx or invalid JSON.
// Must be a general chat/instruct model (not content-safety classifiers).
func OpenRouterFallbackModel() string {
	if m := os.Getenv("AI_OPENROUTER_FALLBACK_MODEL"); m != "" {
		return m
	}
	return "meta-llama/llama-3.3-70b-instruct:free"
}
