package org

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

type createOrgRequest struct {
	Name string `json:"name"`
}

type sendInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type joinTokenRequest struct {
	Token string `json:"token"`
}

type joinCodeRequest struct {
	Code string `json:"code"`
}

func (h *Handler) CreateOrganization(c echo.Context) error {
	var req createOrgRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	result, err := h.service.CreateOrganization(c.Request().Context(), userID, req.Name)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, result)
}

func (h *Handler) ListMyOrgs(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	orgs, err := h.service.ListMyOrgs(c.Request().Context(), userID)
	if err != nil {
		return mapServiceError(c, err)
	}

	if orgs == nil {
		orgs = []OrgSummary{}
	}

	return response.Success(c, http.StatusOK, orgs)
}

func (h *Handler) GetOrganization(c echo.Context) error {
	orgID := c.Param("id")
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	result, err := h.service.GetOrganization(c.Request().Context(), orgID, userID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *Handler) SendEmailInvite(c echo.Context) error {
	orgID := c.Param("id")
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	var req sendInviteRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.SendEmailInvite(c.Request().Context(), orgID, userID, req.Email, req.Role)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, result)
}

func (h *Handler) JoinViaToken(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	var req joinTokenRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.JoinViaToken(c.Request().Context(), userID, req.Token)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *Handler) GenerateInviteCode(c echo.Context) error {
	orgID := c.Param("id")
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	result, err := h.service.GenerateInviteCode(c.Request().Context(), orgID, userID)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, result)
}

func (h *Handler) JoinViaCode(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	var req joinCodeRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.JoinViaCode(c.Request().Context(), userID, req.Code)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func mapServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrInvalidOrgName), errors.Is(err, ErrInvalidEmail), errors.Is(err, ErrInvalidInviteRole):
		return response.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrOrgNotFound), errors.Is(err, ErrInviteNotFound), errors.Is(err, ErrInviteCodeNotFound):
		return response.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrNotOrgMember):
		return response.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInsufficientPermissions):
		return response.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInviteExpired), errors.Is(err, ErrInviteAlreadyAccepted), errors.Is(err, ErrInviteCodeInactive):
		return response.Error(c, http.StatusGone, err.Error())
	case errors.Is(err, ErrAlreadyOrgMember):
		return response.Error(c, http.StatusConflict, err.Error())
	default:
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
