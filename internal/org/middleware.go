package org

import (
	"errors"

	"loglens/internal/middleware"
	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

func RequireOrgAdmin(service *Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get(middleware.ContextKeyUserID).(string)
			if !ok || userID == "" {
				return response.Error(c, 401, "unauthorized")
			}

			orgID := c.Param("id")
			if orgID == "" {
				return response.Error(c, 400, "organization id is required")
			}

			role, err := service.GetOrgMemberRole(c.Request().Context(), orgID, userID)
			if err != nil {
				return mapOrgMiddlewareError(c, err)
			}

			if !IsAdminRole(role) {
				return response.Error(c, 403, ErrInsufficientPermissions.Error())
			}

			c.Set(middleware.ContextKeyOrgRole, role)
			return next(c)
		}
	}
}

func mapOrgMiddlewareError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrOrgNotFound):
		return response.Error(c, 404, err.Error())
	case errors.Is(err, ErrNotOrgMember):
		return response.Error(c, 403, err.Error())
	default:
		return response.Error(c, 500, "internal server error")
	}
}
