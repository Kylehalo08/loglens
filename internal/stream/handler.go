package stream

import (
	"context"
	"net/http"

	"loglens/internal/db"
	"loglens/internal/middleware"
	appsvc "loglens/internal/service"
	"loglens/internal/telemetry"
	"loglens/pkg/response"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type ServiceReader interface {
	GetService(ctx context.Context, orgID, serviceID, userID string) (*appsvc.ServiceResponse, error)
}

type Handler struct {
	redis       *db.RedisStore
	serviceRead ServiceReader
}

func NewHandler(redis *db.RedisStore, serviceRead ServiceReader) *Handler {
	return &Handler{
		redis:       redis,
		serviceRead: serviceRead,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) StreamServiceLogs(c echo.Context) error {
	userID, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return response.Error(c, http.StatusUnauthorized, "unauthorized")
	}

	orgID := c.Param("id")
	serviceID := c.Param("serviceId")

	if _, err := h.serviceRead.GetService(c.Request().Context(), orgID, serviceID, userID); err != nil {
		return response.Error(c, http.StatusForbidden, "not an organization member")
	}

	if h.redis == nil || !h.redis.IsAvailable() {
		return response.Error(c, http.StatusServiceUnavailable, "live stream unavailable")
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	channel := telemetry.ServiceChannel(serviceID)
	pubsub := h.redis.Client().Subscribe(ctx, channel)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "stream error"))
			return nil
		}

		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
			return nil
		}
	}
}
