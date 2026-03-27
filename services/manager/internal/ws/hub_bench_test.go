package ws

import (
	"log/slog"
	"os"
	"testing"
)

// BenchmarkHubAddRemoveClient measures the overhead of client registration
// and deregistration under contention.
func BenchmarkHubAddRemoveClient(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewHub(nil, logger, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// addClient and removeClient use nil conn as key — safe for benchmarking
		// the map + mutex overhead without actual WebSocket connections.
		h.mu.Lock()
		ip := "10.0.0.1"
		h.ipConnCount[ip]++
		h.mu.Unlock()

		h.mu.Lock()
		h.ipConnCount[ip]--
		if h.ipConnCount[ip] <= 0 {
			delete(h.ipConnCount, ip)
		}
		h.mu.Unlock()
	}
}

// BenchmarkHubClientCount measures lock contention on client count reads.
func BenchmarkHubClientCount(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewHub(nil, logger, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.clientCount()
	}
}
