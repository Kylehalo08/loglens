package telemetry

import (
	"errors"
	"net/http"

	"loglens/internal/middleware"
	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) GetLog(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}
	_ = userID

	entry, err := h.repo.GetLogByID(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		c.Param("logId"),
	)
	if err != nil {
		if errors.Is(err, ErrLogNotFound) {
			return response.Error(c, http.StatusNotFound, err.Error())
		}
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}

	return response.Success(c, http.StatusOK, entry)
}

func (h *Handler) Health(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{"status": "ok"})
}
