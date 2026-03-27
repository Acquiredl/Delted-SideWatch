package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/aggregator"
)

// maxConnsPerIP is the maximum number of WebSocket connections allowed per IP address.
const maxConnsPerIP = 5

// Hub manages WebSocket connections and broadcasts pool stats to all clients.
type Hub struct {
	agg            *aggregator.Aggregator
	logger         *slog.Logger
	clients        map[*websocket.Conn]string // conn -> IP
	ipConnCount    map[string]int             // IP -> active connection count
	OriginPatterns []string                   // allowed origin patterns for WebSocket upgrade
	mu             sync.Mutex
}

// NewHub creates a new WebSocket broadcast hub.
func NewHub(agg *aggregator.Aggregator, logger *slog.Logger, originPatterns []string) *Hub {
	return &Hub{
		agg:            agg,
		logger:         logger,
		clients:        make(map[*websocket.Conn]string),
		ipConnCount:    make(map[string]int),
		OriginPatterns: originPatterns,
	}
}

// addClient registers a connection for broadcasts. Returns false if the
// per-IP connection limit has been reached.
func (h *Hub) addClient(conn *websocket.Conn, ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ipConnCount[ip] >= maxConnsPerIP {
		return false
	}
	h.clients[conn] = ip
	h.ipConnCount[ip]++
	h.logger.Debug("ws client connected", "ip", ip, "total", len(h.clients))
	return true
}

// removeClient unregisters a connection.
func (h *Hub) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	if ip, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
		h.ipConnCount[ip]--
		if h.ipConnCount[ip] <= 0 {
			delete(h.ipConnCount, ip)
		}
	}
	h.mu.Unlock()
	h.logger.Debug("ws client disconnected", "total", h.clientCount())
}

func (h *Hub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Run starts the broadcast loop that pushes pool stats every 5 seconds.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.broadcast(ctx)
		}
	}
}

// broadcast fetches current pool stats and sends them to all connected clients.
func (h *Hub) broadcast(ctx context.Context) {
	h.mu.Lock()
	if len(h.clients) == 0 {
		h.mu.Unlock()
		return
	}
	// Snapshot the client set to avoid holding the lock during I/O.
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	stats, err := h.agg.GetPoolStatsCached(ctx)
	if err != nil {
		h.logger.Warn("ws broadcast: failed to get pool stats", "error", err)
		return
	}

	data, err := json.Marshal(stats)
	if err != nil {
		h.logger.Error("ws broadcast: failed to marshal stats", "error", err)
		return
	}

	for _, conn := range clients {
		writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
			h.logger.Debug("ws broadcast: write failed, removing client", "error", err)
			h.removeClient(conn)
			conn.Close(websocket.StatusGoingAway, "write failed")
		}
		cancel()
	}
}
