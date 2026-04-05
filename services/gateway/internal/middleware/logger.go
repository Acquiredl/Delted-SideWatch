package middleware

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack exposes the underlying connection for WebSocket upgrades.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("upstream ResponseWriter does not support hijacking")
	}
	return hj.Hijack()
}

// sanitizePath redacts wallet addresses from miner API paths so that
// access logs never associate a client IP with a specific wallet address.
func sanitizePath(p string) string {
	if strings.HasPrefix(p, "/api/miner/") {
		parts := strings.SplitN(p, "/", 5) // ["", "api", "miner", "ADDRESS", ...]
		if len(parts) >= 4 {
			parts[3] = "[redacted]"
			return strings.Join(parts, "/")
		}
	}
	return p
}

// Logger returns middleware that logs every request using the provided slog.Logger.
// It logs the method, path, status code, duration, request ID, and client IP.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info("request",
				"method", r.Method,
				"path", sanitizePath(r.URL.Path),
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"request_id", r.Header.Get("X-Request-ID"),
				"client_ip", extractIP(r),
			)
		})
	}
}
