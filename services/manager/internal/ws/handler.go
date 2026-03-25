package ws

import (
	"net"
	"net/http"
	"strings"

	"nhooyr.io/websocket"
)

// HandlePoolStats returns an HTTP handler that upgrades to WebSocket
// and holds the connection open for broadcast delivery.
func (h *Hub) HandlePoolStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: h.OriginPatterns,
		})
		if err != nil {
			h.logger.Warn("ws upgrade failed", "error", err)
			return
		}

		// Enforce per-IP connection limit.
		ip := extractIP(r)
		if !h.addClient(conn, ip) {
			h.logger.Warn("ws connection limit reached", "ip", ip)
			conn.Close(websocket.StatusTryAgainLater, "too many connections")
			return
		}
		defer func() {
			h.removeClient(conn)
			conn.Close(websocket.StatusNormalClosure, "")
		}()

		// Cap incoming frame size — we don't expect client messages.
		conn.SetReadLimit(4096)

		// Block until the client disconnects or context is cancelled.
		for {
			_, _, err := conn.Read(r.Context())
			if err != nil {
				return
			}
		}
	}
}

// extractIP extracts the client IP, preferring X-Real-IP (set by nginx).
func extractIP(r *http.Request) string {
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
