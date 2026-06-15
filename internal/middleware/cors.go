package middleware

import (
	"log"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

const corsOriginsEnv = "CORS_ALLOWED_ORIGINS"

var defaultCORSOrigins = []string{
	"http://localhost:5173",
	"http://localhost:5174",
	"http://localhost:3000",
}

// CORS allows browser requests from origins listed in CORS_ALLOWED_ORIGINS
// (comma-separated). When unset, common localhost dev ports are allowed.
func CORS() echo.MiddlewareFunc {
	origins := parseCORSOrigins(os.Getenv(corsOriginsEnv))
	log.Printf("CORS allowed origins: %s", strings.Join(origins, ", "))

	return echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{
			echo.GET,
			echo.HEAD,
			echo.POST,
			echo.PUT,
			echo.PATCH,
			echo.DELETE,
			echo.OPTIONS,
		},
		AllowHeaders: []string{
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderContentType,
		},
		MaxAge: 86400,
	})
}

func parseCORSOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string(nil), defaultCORSOrigins...)
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			origins = append(origins, part)
		}
	}
	if len(origins) == 0 {
		return append([]string(nil), defaultCORSOrigins...)
	}
	return origins
}
