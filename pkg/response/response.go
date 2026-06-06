package response

import "github.com/labstack/echo/v4"

type envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func Success(c echo.Context, statusCode int, data any) error {
	return c.JSON(statusCode, envelope{
		Success: true,
		Data:    data,
	})
}

func Error(c echo.Context, statusCode int, message string) error {
	return c.JSON(statusCode, envelope{
		Success: false,
		Error:   message,
	})
}
