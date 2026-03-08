package manager

import (
	"encoding/json"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/zeelrupapara/seo-rank-guardian/model"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Client struct {
	UserID   uint
	Conn     *websocket.Conn
	Hub      *Hub
	Egress   chan *model.Event
	Shutdown chan struct{}
	done     chan struct{}
}

func NewClient(conn *websocket.Conn, hub *Hub, userID uint) *Client {
	return &Client{
		UserID:   userID,
		Conn:     conn,
		Hub:      hub,
		Egress:   make(chan *model.Event, 64),
		Shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (c *Client) ReadMessages() {
	defer func() {
		close(c.Shutdown)
		close(c.done)
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.Hub.Log.Warnf("WS read error for user %d: %v", c.UserID, err)
			}
			return
		}

		var evt model.Event
		if err := json.Unmarshal(message, &evt); err != nil {
			c.Hub.Log.Warnf("Invalid WS message from user %d: %v", c.UserID, err)
			continue
		}

		handler, ok := c.Hub.getHandler(evt.Type)
		if !ok {
			c.Hub.Log.Warnf("No handler for event type %s from user %d", evt.Type, c.UserID)
			continue
		}

		ctx := &Ctx{
			Client: c,
			Type:   evt.Type,
			Data:   message,
			Event:  &evt,
		}
		if err := handler(ctx); err != nil {
			c.Hub.Log.Errorf("Handler error for event %s: %v", evt.Type, err)
		}
	}
}

func (c *Client) WriteMessages() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.Egress:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				c.Hub.Log.Errorf("Failed to marshal event: %v", err)
				continue
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.Hub.Log.Warnf("WS write error for user %d: %v", c.UserID, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.Shutdown:
			return
		}
	}
}

func (c *Client) Listen() {
	<-c.done
}
