package v1

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/nats-io/nats.go"
	"github.com/zeelrupapara/seo-rank-guardian/model"
	"github.com/zeelrupapara/seo-rank-guardian/pkg/manager"
)

func (h *HttpServer) WebSocketUpgrade(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

func (h *HttpServer) ServeWS(c *websocket.Conn) {
	token := c.Query("token")
	if token == "" {
		c.WriteJSON(fiber.Map{"error": "missing token"})
		c.Close()
		return
	}

	claims, err := h.OAuth2.ValidateAccessToken(token)
	if err != nil {
		c.WriteJSON(fiber.Map{"error": "invalid token"})
		c.Close()
		return
	}

	userID := claims.UserID

	client := manager.NewClient(c.Conn, h.Hub, userID)
	h.Hub.Store(client)

	// Subscribe to per-user NATS subject for WS event delivery
	subject := model.SubjectUserEvents(userID)
	sub, err := h.Nats.SubscribeRaw(subject, func(msg *nats.Msg) {
		var evt model.Event
		if err := json.Unmarshal(msg.Data, &evt); err != nil {
			h.Log.Errorf("Failed to unmarshal NATS event for user %d: %v", userID, err)
			return
		}
		select {
		case client.Egress <- &evt:
		default:
			h.Log.Warnf("Egress channel full for user %d, dropping event", userID)
		}
	})
	if err != nil {
		h.Log.Errorf("Failed to subscribe to NATS for user %d: %v", userID, err)
		h.Hub.Delete(client)
		c.Close()
		return
	}

	h.Log.Infof("WebSocket connected for user %d", userID)

	go client.WriteMessages()
	go client.ReadMessages()

	// Block until shutdown
	client.Listen()

	// Cleanup: unsubscribe NATS, close egress channel, remove from hub
	sub.Unsubscribe()
	close(client.Egress)
	h.Hub.Delete(client)
	h.Log.Infof("WebSocket disconnected for user %d", userID)
}
