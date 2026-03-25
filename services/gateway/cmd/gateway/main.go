package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/gateway/internal/auth"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/gateway/internal/middleware"
)

func main() {
	cfg := LoadConfig()

	// Set up structured JSON logging.
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Parse the manager backend URL.
	managerURL, err := url.Parse(cfg.ManagerURL)
	if err != nil {
		slog.Error("invalid manager URL", "url", cfg.ManagerURL, "error", err)
		os.Exit(1)
	}

	// Create reverse proxy to manager service.
	proxy := httputil.NewSingleHostReverseProxy(managerURL)

	// Parse rate limit RPS from config.
	rps, err := strconv.Atoi(cfg.RateLimitRPS)
	if err != nil {
		slog.Error("invalid rate limit RPS", "value", cfg.RateLimitRPS, "error", err)
		os.Exit(1)
	}

	// Create rate limiter backed by Redis.
	rateLimiter, err := middleware.NewRateLimiter(cfg.RedisURL, rps, logger)
	if err != nil {
		slog.Error("failed to create rate limiter", "error", err)
		os.Exit(1)
	}
	defer rateLimiter.Close()

	mux := http.NewServeMux()

	// Health endpoint for the gateway itself (not proxied, not rate-limited).
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// All other requests are proxied to the manager service.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// Build middleware chain: RequestID -> Logger -> RateLimiter -> JWT Auth -> mux
	// Outermost middleware executes first.
	var handler http.Handler = mux
	handler = auth.Middleware(cfg.JWTSecret, []string{"/api/admin/"}, logger)(handler)
	handler = rateLimiter.Middleware(handler)
	handler = middleware.Logger(logger)(handler)
	handler = middleware.RequestID(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine.
	go func() {
		slog.Info("gateway starting", "port", cfg.APIPort, "manager", cfg.ManagerURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal, then gracefully shut down.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gateway")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("gateway stopped")
}
