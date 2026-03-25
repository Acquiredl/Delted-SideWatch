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
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter provides per-IP rate limiting backed by Redis.
type RateLimiter struct {
	client    *redis.Client
	rps       int
	burstSize int
	logger    *slog.Logger
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
			// If Redis is unavailable, log and allow the request through.
			rl.logger.Warn("rate limiter redis error, allowing request", "error", err, "ip", ip)
			next.ServeHTTP(w, r)
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

// Close shuts down the Redis client connection.
func (rl *RateLimiter) Close() error {
	return rl.client.Close()
}

// extractIP extracts the client IP address from the request, checking
// X-Forwarded-For first, then falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For header (set by nginx/load balancer).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs; take the first one.
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP header.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr, stripping the port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
