package user

import (
	"errors"
	"net/http"

	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	tokens, err := h.service.Register(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusCreated, tokens)
}

func (h *Handler) Login(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	tokens, err := h.service.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, tokens)
}

func (h *Handler) Refresh(c echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	tokens, err := h.service.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, tokens)
}

func (h *Handler) Logout(c echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return mapServiceError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{
		"message": "logged out",
	})
}

func mapServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrInvalidEmail), errors.Is(err, ErrInvalidPassword):
		return response.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrEmailTaken):
		return response.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		return response.Error(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrUserNotFound):
		return response.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalidRefreshToken), errors.Is(err, ErrExpiredRefreshToken):
		return response.Error(c, http.StatusUnauthorized, err.Error())
	default:
		return response.Error(c, http.StatusInternalServerError, "internal server error")
	}
}
