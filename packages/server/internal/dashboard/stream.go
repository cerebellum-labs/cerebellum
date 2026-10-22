package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const (
	// Cloudflare drops a WebSocket that stays quiet for 100 s; a ping
	// every 30 s keeps it open and finds dead clients.
	pingInterval = 30 * time.Second
	writeTimeout = 10 * time.Second
)

// stream sends the dashboard every change as it happens, one JSON event
// per message: {"type": "device.updated", "device": {...}} and the like.
// It is read-only and open to everyone, like the rest of the reading API.
func (h *Handler) stream(c *gin.Context) {
	// The server's read and write timeouts would cut the connection a
	// minute in; a WebSocket lives as long as the page.
	rc := http.NewResponseController(c.Writer)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		h.internalError(c, "clear read deadline", err)
		return
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		h.internalError(c, "clear write deadline", err)
		return
	}

	// Accept answers a failed handshake itself, including an Origin that
	// is neither the API's host nor the dashboard's.
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{OriginPatterns: h.wsOrigins})
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }() // After a clean Close this only reports it was closed.

	msgs, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	// The dashboard sends nothing; CloseRead handles control frames and
	// cancels ctx once the client goes away.
	ctx := conn.CloseRead(c.Request.Context())
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				// The server is shutting down or this client fell behind:
				// either way the dashboard reconnects and reloads.
				_ = conn.Close(websocket.StatusGoingAway, "reconnect")
				return
			}
			if err := h.write(ctx, conn, msg); err != nil {
				return
			}
		case <-ping.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *Handler) write(ctx context.Context, conn *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, msg)
}
