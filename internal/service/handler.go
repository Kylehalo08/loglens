package service

import (
	"errors"
	"net/http"

	"loglens/internal/middleware"
	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type createServiceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateServiceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type createAPIKeyRequest struct {
	Label string `json:"label"`
}

func (h *Handler) CreateService(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	var req createServiceRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.CreateService(
		c.Request().Context(),
		c.Param("id"),
		userID,
		req.Name,
		req.Description,
		c.RealIP(),
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, result)
}

func (h *Handler) ListServices(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	results, err := h.service.ListServices(c.Request().Context(), c.Param("id"), userID)
	if err != nil {
		return mapServiceError(c, err)
	}

	if results == nil {
		results = []ServiceResponse{}
	}

	return response.Success(c, http.StatusOK, results)
}

func (h *Handler) GetService(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	result, err := h.service.GetService(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		userID,
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *Handler) UpdateService(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	var req updateServiceRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.UpdateService(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		userID,
		req.Name,
		req.Description,
		c.RealIP(),
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *Handler) DeleteService(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	if err := h.service.DeleteService(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		userID,
		c.RealIP(),
	); err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{
		"message": "service deleted",
	})
}

func (h *Handler) GenerateAPIKey(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	var req createAPIKeyRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.GenerateAPIKey(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		userID,
		req.Label,
		c.RealIP(),
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, result)
}

func (h *Handler) ListAPIKeys(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	results, err := h.service.ListAPIKeys(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		userID,
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	if results == nil {
		results = []APIKeyResponse{}
	}

	return response.Success(c, http.StatusOK, results)
}

func (h *Handler) RevokeAPIKey(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	result, err := h.service.RevokeAPIKey(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		c.Param("keyId"),
		userID,
		c.RealIP(),
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *Handler) RotateAPIKey(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	result, err := h.service.RotateAPIKey(
		c.Request().Context(),
		c.Param("id"),
		c.Param("serviceId"),
		c.Param("keyId"),
		userID,
		c.RealIP(),
	)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, result)
}

func mapServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrInvalidServiceName), errors.Is(err, ErrServiceNameTooLong):
		return response.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrServiceNotFound), errors.Is(err, ErrAPIKeyNotFound):
		return response.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrNotOrgMember), errors.Is(err, ErrInsufficientPermissions):
		return response.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrDuplicateServiceName):
		return response.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrAPIKeyAlreadyRevoked):
		return response.Error(c, http.StatusGone, err.Error())
	default:
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
