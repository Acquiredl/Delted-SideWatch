package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/aggregator"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/cache"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/events"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/metrics"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/p2pool"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/scanner"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/ws"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/db"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/monerod"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/p2poolclient"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL.
	pool, err := db.New(ctx, cfg.PostgresConnString())
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Run database migrations.
	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Connect to Redis.
	cacheStore, err := cache.New(cfg.RedisURL, slog.Default())
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer cacheStore.Close()

	// Create P2Pool client + service.
	p2poolClient := p2poolclient.New(cfg.P2PoolAPIURL, slog.Default())
	p2poolService := p2pool.NewService(p2poolClient, cfg.P2PoolSidechain, slog.Default())

	// Create monerod client.
	monerodClient := monerod.New(cfg.MonerodURL, slog.Default())

	// Create aggregator.
	agg := aggregator.New(pool, cfg.P2PoolSidechain, slog.Default())

	// Create and start indexer.
	indexer := p2pool.NewIndexer(p2poolService, pool, 30*time.Second, slog.Default())
	go func() {
		if err := indexer.Run(ctx); err != nil {
			slog.Error("indexer stopped", "error", err)
		}
	}()

	// Create and start timeseries builder.
	tsBuilder := aggregator.NewTimeseriesBuilder(pool, cfg.P2PoolSidechain, slog.Default())
	go func() {
		if err := tsBuilder.Run(ctx); err != nil {
			slog.Error("timeseries builder stopped", "error", err)
		}
	}()

	// Create scanner + block listener.
	scn := scanner.NewScanner(monerodClient, pool, 10, slog.Default())
	blockListener := events.NewBlockListener(cfg.MonerodZMQURL, monerodClient, slog.Default())
	blockListener.OnBlock(func(height uint64) {
		if err := scn.HandleNewBlock(ctx, height); err != nil {
			slog.Error("scanner error", "height", height, "error", err)
		}
	})
	go func() {
		if err := blockListener.Run(ctx); err != nil {
			slog.Error("block listener stopped", "error", err)
		}
	}()

	// Create and start WebSocket broadcast hub.
	wsHub := ws.NewHub(agg, slog.Default())
	go wsHub.Run(ctx)

	// Set up HTTP routes.
	mux := http.NewServeMux()
	RegisterRoutes(mux, pool, agg, cacheStore, wsHub)

	// Start metrics server on separate port.
	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("GET /metrics", metrics.Handler())
		slog.Info("metrics server starting", "port", cfg.MetricsPort)
		if err := http.ListenAndServe(":"+cfg.MetricsPort, metricsMux); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine.
	go func() {
		slog.Info("manager starting", "port", cfg.APIPort, "sidechain", cfg.P2PoolSidechain)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal, then gracefully shut down.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down manager")
	cancel() // Signal background goroutines to stop.

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("manager stopped")
}
