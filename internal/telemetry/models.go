package telemetry

import (
	"encoding/json"
	"time"
)

const MaxMessageBytes = 65536

var ValidSeverities = map[string]struct{}{
	"DEBUG": {},
	"INFO":  {},
	"WARN":  {},
	"ERROR": {},
	"FATAL": {},
}

type IngestRequest struct {
	Timestamp *time.Time     `json:"timestamp"`
	Severity  string         `json:"severity"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
}

type LogEntry struct {
	ID         string         `json:"id"`
	OrgID      string         `json:"org_id"`
	ServiceID  string         `json:"service_id"`
	Timestamp  time.Time      `json:"timestamp"`
	Severity   string         `json:"severity"`
	Message    string         `json:"message"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	IngestedAt time.Time      `json:"ingested_at"`
}

func (e LogEntry) MarshalJSON() ([]byte, error) {
	type alias LogEntry
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	return json.Marshal(alias(e))
}

func IsValidSeverity(severity string) bool {
	_, ok := ValidSeverities[severity]
	return ok
}

func ServiceChannel(serviceID string) string {
	return "logs:service:" + serviceID
}

type SearchFilters struct {
	OrgID      string
	ServiceIDs []string
	Severities []string
	From       *time.Time
	To         *time.Time
	Query      string
	Page       int
	Limit      int
}

type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type SearchResult struct {
	Logs       []LogEntry `json:"logs"`
	Pagination Pagination `json:"pagination"`
}
