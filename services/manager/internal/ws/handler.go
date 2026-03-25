package ws

import (
	"net/http"

	"nhooyr.io/websocket"
)

// HandlePoolStats returns an HTTP handler that upgrades to WebSocket
// and holds the connection open for broadcast delivery.
func (h *Hub) HandlePoolStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// Allow all origins in dev; nginx handles origin checks in production.
			InsecureSkipVerify: true,
		})
		if err != nil {
			h.logger.Warn("ws upgrade failed", "error", err)
			return
		}

		h.addClient(conn)
		defer func() {
			h.removeClient(conn)
			conn.Close(websocket.StatusNormalClosure, "")
		}()

		// Block until the client disconnects or context is cancelled.
		// We don't expect any client-to-server messages, so just drain reads.
		for {
			_, _, err := conn.Read(r.Context())
			if err != nil {
				return
			}
		}
	}
}
