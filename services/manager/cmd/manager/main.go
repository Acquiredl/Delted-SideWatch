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
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/fund"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/metrics"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/nodepool"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/p2pool"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/scanner"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/subscription"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/ws"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/db"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/monerod"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/p2poolclient"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/walletrpc"
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
		cancel()
		slog.Error("failed to connect to postgres", "error", err)
		return
	}
	defer pool.Close()

	// Run database migrations.
	if err := db.Migrate(ctx, pool); err != nil {
		cancel()
		slog.Error("failed to run migrations", "error", err)
		return
	}

	// Connect to Redis.
	cacheStore, err := cache.New(cfg.RedisURL, slog.Default())
	if err != nil {
		cancel()
		slog.Error("failed to connect to redis", "error", err)
		return
	}
	defer func() { _ = cacheStore.Close() }()

	// Create P2Pool client + service.
	p2poolClient := p2poolclient.New(cfg.P2PoolAPIURL, slog.Default())
	p2poolService := p2pool.NewService(p2poolClient, cfg.P2PoolSidechain, slog.Default())

	// Create monerod client.
	monerodClient := monerod.New(cfg.MonerodURL, slog.Default())

	// Create aggregator.
	agg := aggregator.New(pool, cacheStore, cfg.P2PoolSidechain, slog.Default())

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

	// Create price oracle for fiat conversion.
	priceOracle := scanner.NewPriceOracle(slog.Default(), cfg.CoingeckoURL)

	// Create scanner + block listener.
	scn := scanner.NewScanner(monerodClient, pool, priceOracle, 10, slog.Default())
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

	// Create subscription service and scanner.
	subSvc := subscription.NewService(pool, cacheStore, slog.Default())
	var subScn *subscription.Scanner
	if cfg.WalletRPCURL != "" {
		walletClient := walletrpc.New(cfg.WalletRPCURL, slog.Default())
		subScn = subscription.NewScanner(walletClient, pool, priceOracle, subscription.ScannerConfig{
			ConfirmDepth: 10,
			MinUSD:       cfg.SubscriptionMinUSD,
			DurationDays: cfg.SubscriptionDurationDays,
			GraceHours:   cfg.SubscriptionGraceHours,
			FundGoalUSD:  cfg.FundGoalUSD,
			InfraCostUSD: cfg.InfraCostUSD,
		}, slog.Default())
		go func() {
			if err := subScn.Run(ctx); err != nil {
				slog.Error("subscription scanner stopped", "error", err)
			}
		}()
	} else {
		slog.Warn("WALLET_RPC_URL not set, subscription payment scanning disabled")
	}

	// Create fund service.
	fundSvc := fund.NewService(pool, cfg.FundGoalUSD, cfg.InfraCostUSD, slog.Default())

	// Create node pool manager and start health checker.
	nodePool := nodepool.New(pool, nodepool.Config{
		StratumHost: cfg.StratumHost,
		OnionURL:    cfg.OnionStratumURL,
	}, slog.Default())
	go func() {
		if err := nodePool.RunHealthChecker(ctx); err != nil {
			slog.Error("node health checker stopped", "error", err)
		}
	}()

	// Create and start WebSocket broadcast hub.
	// Origin patterns control which domains can open WebSocket connections.
	// In production, nginx proxies requests so the origin matches the site domain.
	wsOrigins := []string{}
	if origin := getEnvOrDefault("WS_ALLOWED_ORIGIN", ""); origin != "" {
		wsOrigins = append(wsOrigins, origin)
	}
	wsHub := ws.NewHub(agg, slog.Default(), wsOrigins)
	go wsHub.Run(ctx)

	// Set up HTTP routes.
	mux := http.NewServeMux()
	RegisterRoutes(mux, pool, agg, cacheStore, wsHub, priceOracle, subSvc, subScn, p2poolService, fundSvc, nodePool, cfg.AdminToken)

	// Wrap mux with tier middleware so all handlers can read subscription tier from context.
	handler := subscription.TierMiddleware(subSvc, logger)(mux)

	// Start metrics server on separate port with timeouts.
	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("GET /metrics", metrics.Handler())
		metricsSrv := &http.Server{
			Addr:         ":" + cfg.MetricsPort,
			Handler:      metricsMux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
		}
		slog.Info("metrics server starting", "port", cfg.MetricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      handler,
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
