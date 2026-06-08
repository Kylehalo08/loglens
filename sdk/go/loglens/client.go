package loglens

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Client struct {
	ingestURL  string
	apiKey     string
	httpClient *http.Client
}

type ingestPayload struct {
	Timestamp *time.Time     `json:"timestamp,omitempty"`
	Severity  string         `json:"severity"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func NewClient(apiKey string) *Client {
	ingestURL := os.Getenv("LOGLENS_INGESTOR_URL")
	if ingestURL == "" {
		ingestURL = os.Getenv("INGESTOR_URL")
	}
	if ingestURL == "" {
		ingestURL = "http://localhost:8081"
	}

	return &Client{
		ingestURL: ingestURL,
		apiKey:    apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Log(ctx context.Context, severity, message string, metadata map[string]any) error {
	payload := ingestPayload{
		Severity: severity,
		Message:  message,
		Metadata: metadata,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ingestURL+"/v1/logs", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ingest failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Debug(ctx context.Context, message string, metadata map[string]any) error {
	return c.Log(ctx, "DEBUG", message, metadata)
}

func (c *Client) Info(ctx context.Context, message string, metadata map[string]any) error {
	return c.Log(ctx, "INFO", message, metadata)
}

func (c *Client) Warn(ctx context.Context, message string, metadata map[string]any) error {
	return c.Log(ctx, "WARN", message, metadata)
}

func (c *Client) Error(ctx context.Context, message string, metadata map[string]any) error {
	return c.Log(ctx, "ERROR", message, metadata)
}

func (c *Client) Fatal(ctx context.Context, message string, metadata map[string]any) error {
	return c.Log(ctx, "FATAL", message, metadata)
}
