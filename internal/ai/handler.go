package ai

import (
	"net/http"
	"time"

	"loglens/internal/middleware"
	"loglens/internal/telemetry"
	"loglens/pkg/response"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	orchestrator *Orchestrator
	logRepo      *telemetry.Repository
}

func NewHandler(orchestrator *Orchestrator, logRepo *telemetry.Repository) *Handler {
	return &Handler{orchestrator: orchestrator, logRepo: logRepo}
}

// --- FR13: NL -> structured search filters, then run search

type nlqRequest struct {
	Question string `json:"question"`
	Page     int    `json:"page,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type nlqAIResponse struct {
	ServiceIDs []string `json:"service_ids"`
	Severity   []string `json:"severity"`
	From       *string  `json:"from"`
	To         *string  `json:"to"`
	Q          string   `json:"q"`
	Limit      int      `json:"limit"`
}

type nlqResponse struct {
	AI struct {
		Provider Provider `json:"provider"`
		Model    string   `json:"model"`
		Query    nlqAIResponse `json:"query"`
	} `json:"ai"`
	Result *telemetry.SearchResult `json:"result"`
}

func (h *Handler) NLQ(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}
	_ = userID

	orgID := c.Param("id")
	if _, err := uuid.Parse(orgID); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid organization id")
	}

	var req nlqRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}
	if req.Question == "" {
		return response.Error(c, http.StatusBadRequest, "question is required")
	}

	var aiOut nlqAIResponse
	run, err := h.orchestrator.RunJSON(
		c.Request().Context(),
		GenerateRequest{UseCase: "fr13_nlq", Model: "gemini-2.5-flash", Prompt: PromptNLQ(req.Question)},
		OpenRouterFallbackModel(),
		&aiOut,
	)
	if err != nil {
		return response.Error(c, http.StatusServiceUnavailable, "ai unavailable")
	}

	// Convert to telemetry.SearchFilters
	filters := telemetry.SearchFilters{
		OrgID:      orgID,
		ServiceIDs: aiOut.ServiceIDs,
		Severities: aiOut.Severity,
		Query:      aiOut.Q,
		Page:       1,
		Limit:      telemetry.DefaultSearchLimit,
	}
	if req.Page > 0 {
		filters.Page = req.Page
	}
	if req.Limit > 0 {
		filters.Limit = req.Limit
	} else if aiOut.Limit > 0 {
		filters.Limit = aiOut.Limit
	}

	if aiOut.From != nil && *aiOut.From != "" {
		if t, err := time.Parse(time.RFC3339, *aiOut.From); err == nil {
			filters.From = &t
		}
	}
	if aiOut.To != nil && *aiOut.To != "" {
		if t, err := time.Parse(time.RFC3339, *aiOut.To); err == nil {
			filters.To = &t
		}
	}

	result, err := h.logRepo.SearchLogs(c.Request().Context(), filters)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}

	out := nlqResponse{Result: result}
	out.AI.Provider = run.Provider
	out.AI.Model = run.Model
	out.AI.Query = aiOut

	return response.Success(c, http.StatusOK, out)
}

// --- FR14: summarization over provided logs

type summarizeRequest struct {
	Window string             `json:"window"`
	Logs   []SummarizeInputLog `json:"logs"`
}

type summarizeResponse struct {
	Summary     string `json:"summary"`
	Highlights  []string `json:"highlights"`
	TopErrors   []struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	} `json:"top_errors"`
	Confidence float64 `json:"confidence"`
}

func (h *Handler) Summarize(c echo.Context) error {
	var req summarizeRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}
	if len(req.Logs) == 0 {
		return response.Error(c, http.StatusBadRequest, "logs are required")
	}
	prompt, err := PromptSummarize(req.Window, req.Logs)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}

	var aiOut summarizeResponse
	_, err = h.orchestrator.RunJSON(
		c.Request().Context(),
		GenerateRequest{UseCase: "fr14_summarize", Model: "gemini-2.5-flash", Prompt: prompt},
		OpenRouterFallbackModel(),
		&aiOut,
	)
	if err != nil {
		return response.Error(c, http.StatusServiceUnavailable, "ai unavailable")
	}
	return response.Success(c, http.StatusOK, aiOut)
}

// --- FR15: investigation over provided logs

type investigateRequest struct {
	Question string             `json:"question"`
	Logs     []SummarizeInputLog `json:"logs"`
}

type investigateResponse struct {
	Hypothesis    string   `json:"hypothesis"`
	Evidence      []string `json:"evidence"`
	NextSteps     []string `json:"next_steps"`
	RelatedLogIDs []string `json:"related_log_ids"`
	Confidence    float64  `json:"confidence"`
}

func (h *Handler) Investigate(c echo.Context) error {
	var req investigateRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}
	if req.Question == "" {
		return response.Error(c, http.StatusBadRequest, "question is required")
	}
	if len(req.Logs) == 0 {
		return response.Error(c, http.StatusBadRequest, "logs are required")
	}

	prompt, err := PromptInvestigate(req.Question, req.Logs)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}

	var aiOut investigateResponse
	_, err = h.orchestrator.RunJSON(
		c.Request().Context(),
		GenerateRequest{UseCase: "fr15_investigate", Model: "gemini-2.5-flash", Prompt: prompt},
		OpenRouterFallbackModel(),
		&aiOut,
	)
	if err != nil {
		return response.Error(c, http.StatusServiceUnavailable, "ai unavailable")
	}
	return response.Success(c, http.StatusOK, aiOut)
}

