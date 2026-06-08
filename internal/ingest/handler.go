package ingest

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"loglens/internal/auth"
	"loglens/internal/telemetry"
	"loglens/pkg/response"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	keyCache *auth.APIKeyCache
	producer *telemetry.Producer
}

func NewHandler(keyCache *auth.APIKeyCache, producer *telemetry.Producer) *Handler {
	return &Handler{
		keyCache: keyCache,
		producer: producer,
	}
}

func (h *Handler) IngestLog(c echo.Context) error {
	rawKey, err := bearerToken(c.Request().Header.Get("Authorization"))
	if err != nil {
		return response.Error(c, http.StatusUnauthorized, err.Error())
	}

	resolved, err := h.keyCache.Resolve(c.Request().Context(), rawKey)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidAPIKeyFormat), errors.Is(err, auth.ErrInvalidAPIKey):
			return response.Error(c, http.StatusUnauthorized, "invalid api key")
		case errors.Is(err, auth.ErrAPIKeyRevoked):
			return response.Error(c, http.StatusUnauthorized, "api key revoked")
		default:
			return response.Error(c, http.StatusUnauthorized, "invalid api key")
		}
	}

	var req telemetry.IngestRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	severity := strings.ToUpper(strings.TrimSpace(req.Severity))
	if !telemetry.IsValidSeverity(severity) {
		return response.Error(c, http.StatusBadRequest, "invalid severity")
	}

	message := req.Message
	if message == "" {
		return response.Error(c, http.StatusBadRequest, "message is required")
	}
	if len(message) > telemetry.MaxMessageBytes {
		return response.Error(c, http.StatusBadRequest, "message exceeds 64KB limit")
	}

	timestamp := time.Now().UTC()
	if req.Timestamp != nil {
		timestamp = req.Timestamp.UTC()
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	now := time.Now().UTC()
	entry := telemetry.LogEntry{
		ID:         uuid.NewString(),
		OrgID:      resolved.OrgID,
		ServiceID:  resolved.ServiceID,
		Timestamp:  timestamp,
		Severity:   severity,
		Message:    message,
		Metadata:   metadata,
		IngestedAt: now,
	}

	if err := h.producer.Publish(c.Request().Context(), entry); err != nil {
		return response.Error(c, http.StatusServiceUnavailable, "failed to queue log")
	}

	return response.Success(c, http.StatusOK, map[string]string{
		"id": entry.ID,
	})
}

func (h *Handler) Health(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{"status": "ok"})
}

func bearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		return "", errors.New("invalid authorization header")
	}
	return token, nil
}
