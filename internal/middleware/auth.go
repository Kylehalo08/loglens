package middleware

import (
	"strings"

	"loglens/internal/auth"
	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

const (
	ContextKeyUserID = "userID"
	ContextKeyRole   = "role"
	ContextKeyOrgRole = "orgRole"
)

func RequireAuth(tokens auth.TokenManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return response.Error(c, 401, "missing authorization header")
			}

			tokenString, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || tokenString == "" {
				return response.Error(c, 401, "invalid authorization header")
			}

			claims, err := tokens.VerifyAccessToken(tokenString)
			if err != nil {
				return response.Error(c, 401, "unauthorized")
			}

			c.Set(ContextKeyUserID, claims.UserID)
			c.Set(ContextKeyRole, claims.Role)
			return next(c)
		}
	}
}
