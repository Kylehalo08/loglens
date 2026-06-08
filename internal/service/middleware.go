package service

import (
	"context"
	"errors"

	"loglens/internal/middleware"
	"loglens/internal/org"
	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

type OrgAccess interface {
	GetOrgMemberRole(ctx context.Context, orgID, userID string) (string, error)
}

func IsDeveloperRole(role string) bool {
	return role == org.RoleOwner || role == org.RoleAdmin || role == org.RoleDeveloper
}

func RequireOrgMember(access OrgAccess) echo.MiddlewareFunc {
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

			role, err := access.GetOrgMemberRole(c.Request().Context(), orgID, userID)
			if err != nil {
				return mapMiddlewareError(c, err)
			}

			c.Set(middleware.ContextKeyOrgRole, role)
			return next(c)
		}
	}
}

func RequireOrgDeveloper(access OrgAccess) echo.MiddlewareFunc {
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

			role, err := access.GetOrgMemberRole(c.Request().Context(), orgID, userID)
			if err != nil {
				return mapMiddlewareError(c, err)
			}

			if !IsDeveloperRole(role) {
				return response.Error(c, 403, ErrInsufficientPermissions.Error())
			}

			c.Set(middleware.ContextKeyOrgRole, role)
			return next(c)
		}
	}
}

func mapMiddlewareError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, org.ErrOrgNotFound):
		return response.Error(c, 404, err.Error())
	case errors.Is(err, org.ErrNotOrgMember), errors.Is(err, ErrNotOrgMember):
		return response.Error(c, 403, err.Error())
	default:
		return response.Error(c, 500, "internal server error")
	}
}
