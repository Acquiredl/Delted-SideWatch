package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// memEntry tracks in-memory rate limit state for a single IP.
type memEntry struct {
	count  int
	window int64
}

// RateLimiter provides per-IP rate limiting backed by Redis with an
// in-memory fallback when Redis is unavailable.
type RateLimiter struct {
	client    *redis.Client
	rps       int
	burstSize int
	logger    *slog.Logger

	// In-memory fallback when Redis is unreachable.
	memMu    sync.Mutex
	memStore map[string]*memEntry
}

// NewRateLimiter creates a new RateLimiter that connects to Redis at the given
// address and enforces the specified requests-per-second limit. Burst size is
// set to 2x rps.
func NewRateLimiter(redisAddr string, rps int, logger *slog.Logger) (*RateLimiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis at %s: %w", redisAddr, err)
	}

	logger.Info("rate limiter connected to redis", "addr", redisAddr, "rps", rps)

	return &RateLimiter{
		client:    client,
		rps:       rps,
		burstSize: rps * 2,
		logger:    logger,
		memStore:  make(map[string]*memEntry),
	}, nil
}

// Middleware returns an HTTP middleware that enforces per-IP rate limiting
// using a sliding window counter in Redis. On 429, it returns a JSON error
// response with a Retry-After header.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exempt WebSocket upgrade requests from rate limiting.
		if strings.HasPrefix(r.URL.Path, "/ws/") {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractIP(r)
		window := time.Now().Unix()
		key := fmt.Sprintf("ratelimit:%s:%d", ip, window)

		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()

		count, err := rl.client.Incr(ctx, key).Result()
		if err != nil {
			// Redis unavailable — fall back to in-memory rate limiting.
			rl.logger.Warn("rate limiter redis error, using in-memory fallback", "error", err, "ip", ip)
			if rl.inMemoryCheck(ip, window) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}

		// Set expiry on first increment so keys clean themselves up.
		if count == 1 {
			rl.client.Expire(ctx, key, 2*time.Second)
		}

		if int(count) > rl.burstSize {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			rl.logger.Warn("rate limit exceeded", "ip", ip, "count", count, "limit", rl.burstSize)
			return
		}

		// Set rate limit headers for client visibility.
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.burstSize))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(rl.burstSize-int(count)))

		next.ServeHTTP(w, r)
	})
}

// inMemoryCheck performs a simple per-IP rate limit check using an in-memory
// map. Returns true if the request is allowed, false if rate limited.
func (rl *RateLimiter) inMemoryCheck(ip string, window int64) bool {
	rl.memMu.Lock()
	defer rl.memMu.Unlock()

	entry, ok := rl.memStore[ip]
	if !ok || entry.window != window {
		rl.memStore[ip] = &memEntry{count: 1, window: window}
		return true
	}
	entry.count++
	return entry.count <= rl.burstSize
}

// Close shuts down the Redis client connection.
func (rl *RateLimiter) Close() error {
	return rl.client.Close()
}

// extractIP extracts the client IP address from the request. It prefers
// X-Real-IP (set by nginx to $remote_addr, not spoofable). X-Forwarded-For
// is only used as a fallback and the rightmost IP is taken since leftmost
// entries can be set by the client.
func extractIP(r *http.Request) string {
	// Prefer X-Real-IP — nginx sets this to $remote_addr (trusted).
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fallback to X-Forwarded-For. Take the rightmost (last) IP which is
	// the one appended by the nearest trusted proxy.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[len(parts)-1])
		if ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr, stripping the port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
