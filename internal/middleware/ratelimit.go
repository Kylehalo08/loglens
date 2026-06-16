package middleware

import (
	"errors"
	"time"

	"loglens/internal/ratelimit"
	"loglens/pkg/response"

	"github.com/labstack/echo/v4"
)

func RateLimitByIP(limiter *ratelimit.Limiter, prefix string, limit int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := limiter.Allow(c.Request().Context(), ratelimit.IPMinuteKey(prefix, c.RealIP()), limit, time.Minute); err != nil {
				return mapRateLimitError(c, err)
			}
			return next(c)
		}
	}
}

func RateLimitByUser(limiter *ratelimit.Limiter, prefix string, limit int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get(ContextKeyUserID).(string)
			if !ok || userID == "" {
				return next(c)
			}
			if err := limiter.Allow(c.Request().Context(), ratelimit.UserMinuteKey(prefix, userID), limit, time.Minute); err != nil {
				return mapRateLimitError(c, err)
			}
			return next(c)
		}
	}
}

func mapRateLimitError(c echo.Context, err error) error {
	if errors.Is(err, ratelimit.ErrRateLimited) {
		return response.Error(c, 429, "rate limit exceeded")
	}
	return response.Error(c, 500, "internal server error")
}
