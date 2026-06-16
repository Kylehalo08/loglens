package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func PromptNLQ(question string) string {
	// Keep prompt short. Hard-require JSON only.
	return strings.TrimSpace(fmt.Sprintf(`
Return ONLY valid JSON. No markdown. No extra keys.

Schema:
{"service_ids":["uuid"],"severity":["DEBUG|INFO|WARN|ERROR|FATAL"],"from":"RFC3339|null","to":"RFC3339|null","q":"string","limit":100}

Rules:
- service_ids can be empty array
- severity can be empty array
- from/to may be null
- q should be a short keyword query (or empty string)
- limit 1..1000

Question: %q
`, question))
}

type SummarizeInputLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	ServiceID string    `json:"service_id"`
}

func PromptSummarize(window string, logs []SummarizeInputLog) (string, error) {
	b, err := json.Marshal(logs)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fmt.Sprintf(`
Return ONLY valid JSON. No markdown. No extra keys.

Schema:
{"summary":"string","highlights":["string"],"top_errors":[{"message":"string","count":0}],"confidence":0.0}

Context: Summarize logs for window %q.
Logs (JSON array):
%s
`, window, string(b))), nil
}

func PromptInvestigate(question string, logs []SummarizeInputLog) (string, error) {
	b, err := json.Marshal(logs)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(fmt.Sprintf(`
Return ONLY valid JSON. No markdown. No extra keys.

Schema:
{"hypothesis":"string","evidence":["string"],"next_steps":["string"],"related_log_ids":["uuid"],"confidence":0.0}

Question: %q
Logs (JSON array):
%s
`, question, string(b))), nil
}

