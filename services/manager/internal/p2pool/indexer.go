package p2pool

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/metrics"
)

// Indexer polls the P2Pool data-api and records stats into Postgres.
// It replaces the share-by-share indexing with aggregate pool stats
// and local stratum worker hashrates.
type Indexer struct {
	service        *Service
	pool           *pgxpool.Pool
	interval       time.Duration
	lastBlockFound uint64 // tracks last known found block height
	logger         *slog.Logger
}

// NewIndexer creates a new P2Pool indexer.
func NewIndexer(service *Service, pool *pgxpool.Pool, interval time.Duration, logger *slog.Logger) *Indexer {
	return &Indexer{
		service:  service,
		pool:     pool,
		interval: interval,
		logger:   logger.With(slog.String("component", "p2pool-indexer")),
	}
}

// Run starts the indexer polling loop. It blocks until ctx is cancelled.
func (idx *Indexer) Run(ctx context.Context) error {
	idx.logger.Info("starting p2pool indexer",
		slog.String("sidechain", idx.service.Sidechain()),
		slog.Duration("interval", idx.interval),
	)

	// Run once immediately before starting the ticker.
	idx.runCycle(ctx)

	ticker := time.NewTicker(idx.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			idx.logger.Info("p2pool indexer stopping")
			return ctx.Err()
		case <-ticker.C:
			idx.runCycle(ctx)
		}
	}
}

// runCycle performs one indexing cycle: pool stats + local workers.
func (idx *Indexer) runCycle(ctx context.Context) {
	start := time.Now()

	if err := idx.indexPoolStats(ctx); err != nil {
		idx.logger.Error("failed to index pool stats", slog.String("error", err.Error()))
		metrics.IndexerErrors.Inc()
	}

	workerCount, err := idx.indexLocalWorkers(ctx)
	if err != nil {
		idx.logger.Error("failed to index local workers", slog.String("error", err.Error()))
		metrics.IndexerErrors.Inc()
	}

	elapsed := time.Since(start)
	metrics.IndexerPollDuration.Observe(elapsed.Seconds())

	idx.logger.Info("indexing cycle complete",
		slog.Int("workers_updated", workerCount),
		slog.Duration("elapsed", elapsed),
	)
}

// indexPoolStats fetches pool/stats, detects found blocks, records a snapshot,
// and updates Prometheus gauges.
func (idx *Indexer) indexPoolStats(ctx context.Context) error {
	stats, err := idx.service.FetchPoolStats(ctx)
	if err != nil {
		return fmt.Errorf("fetching pool stats: %w", err)
	}

	ps := stats.PoolStatistics

	// Update Prometheus gauges.
	metrics.PoolHashrate.Set(float64(ps.HashRate))
	metrics.PoolMiners.Set(float64(ps.Miners))

	// Detect new found block.
	if ps.LastBlockFound > 0 && ps.LastBlockFound != idx.lastBlockFound {
		idx.logger.Info("new block found by pool",
			slog.Uint64("main_height", ps.LastBlockFound),
			slog.Uint64("sidechain_height", ps.SidechainHeight),
		)
		_, err := idx.pool.Exec(ctx,
			`INSERT INTO p2pool_blocks (main_height, main_hash, sidechain_height, coinbase_reward, found_at)
			 VALUES ($1, $2, $3, $4, NOW())
			 ON CONFLICT (main_height) DO NOTHING`,
			ps.LastBlockFound, "", ps.SidechainHeight, 0)
		if err != nil {
			idx.logger.Error("failed to insert found block", slog.Uint64("height", ps.LastBlockFound), slog.String("error", err.Error()))
		} else {
			metrics.BlocksFound.Inc()
		}
		idx.lastBlockFound = ps.LastBlockFound
	}

	// Record pool stats snapshot.
	_, err = idx.pool.Exec(ctx,
		`INSERT INTO pool_stats_snapshots (sidechain, pool_hashrate, pool_miners, sidechain_height, sidechain_difficulty, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		idx.service.Sidechain(), ps.HashRate, ps.Miners, ps.SidechainHeight, ps.SidechainDifficulty)
	if err != nil {
		return fmt.Errorf("inserting pool stats snapshot: %w", err)
	}

	return nil
}

// indexLocalWorkers fetches local/stratum and upserts worker hashrates
// into the miner_hashrate table.
func (idx *Indexer) indexLocalWorkers(ctx context.Context) (int, error) {
	stratum, err := idx.service.FetchLocalStratum(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetching local stratum: %w", err)
	}

	if len(stratum.Workers) == 0 {
		return 0, nil
	}

	sidechain := idx.service.Sidechain()
	bucketStart := truncateToBucket(time.Now().UTC())
	count := 0

	for _, w := range stratum.Workers {
		if w.Address == "" {
			continue
		}

		_, err := idx.pool.Exec(ctx,
			`INSERT INTO miner_hashrate (miner_address, sidechain, hashrate, bucket_time)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (miner_address, sidechain, bucket_time)
			 DO UPDATE SET hashrate = EXCLUDED.hashrate`,
			w.Address, sidechain, w.Hashrate, bucketStart)
		if err != nil {
			return count, fmt.Errorf("upserting hashrate for worker %s: %w", w.Address, err)
		}

		metrics.MinerHashrate.WithLabelValues(w.Address, sidechain).Set(float64(w.Hashrate))
		count++
	}

	return count, nil
}

// truncateToBucket truncates a time to the nearest 15-minute boundary.
func truncateToBucket(t time.Time) time.Time {
	minutes := t.Minute()
	bucketMinute := (minutes / 15) * 15
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), bucketMinute, 0, 0, t.Location())
}
