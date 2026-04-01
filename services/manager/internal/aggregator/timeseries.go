package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/metrics"
)

const (
	// bucketInterval is the duration of each hashrate timeseries bucket.
	bucketInterval = 15 * time.Minute

	// bucketSeconds is the number of seconds in one bucket (used for H/s calculation).
	bucketSeconds = 900
)

// TimeseriesBuilder aggregates raw shares into hashrate timeseries buckets.
type TimeseriesBuilder struct {
	pool      *pgxpool.Pool
	sidechain string
	logger    *slog.Logger
}

// NewTimeseriesBuilder creates a new TimeseriesBuilder.
func NewTimeseriesBuilder(pool *pgxpool.Pool, sidechain string, logger *slog.Logger) *TimeseriesBuilder {
	return &TimeseriesBuilder{
		pool:      pool,
		sidechain: sidechain,
		logger:    logger,
	}
}

// Run starts the periodic rollup loop. It runs one rollup immediately, then
// every 15 minutes until the context is cancelled. Data retention pruning
// runs once per day.
func (tb *TimeseriesBuilder) Run(ctx context.Context) error {
	tb.logger.Info("starting timeseries builder", "sidechain", tb.sidechain, "interval", bucketInterval)

	// Run an initial rollup immediately.
	if err := tb.rollup(ctx); err != nil {
		tb.logger.Error("initial rollup failed", "err", err)
	}

	rollupTicker := time.NewTicker(bucketInterval)
	defer rollupTicker.Stop()

	pruneTicker := time.NewTicker(24 * time.Hour)
	defer pruneTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			tb.logger.Info("timeseries builder stopping")
			return ctx.Err()
		case <-rollupTicker.C:
			if err := tb.rollup(ctx); err != nil {
				tb.logger.Error("rollup failed", "err", err)
			}
		case <-pruneTicker.C:
			if err := tb.pruneRetention(ctx); err != nil {
				tb.logger.Error("retention pruning failed", "err", err)
			}
		}
	}
}

// TruncateToBucket truncates a time to the nearest 15-minute boundary.
// Examples: 14:07 -> 14:00, 14:16 -> 14:15, 14:31 -> 14:30, 14:46 -> 14:45
func TruncateToBucket(t time.Time) time.Time {
	minutes := t.Minute()
	bucketMinute := (minutes / 15) * 15
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), bucketMinute, 0, 0, t.Location())
}

// CalculateHashrate converts total difficulty over a bucket into H/s.
// hashrate = totalDifficulty / bucketSeconds
func CalculateHashrate(totalDifficulty uint64) uint64 {
	if totalDifficulty == 0 {
		return 0
	}
	return totalDifficulty / bucketSeconds
}

// rollup performs one aggregation cycle. It computes the current 15-minute
// bucket, sums share difficulty per miner within that window, calculates
// hashrate, and upserts the result into miner_hashrate.
func (tb *TimeseriesBuilder) rollup(ctx context.Context) error {
	now := time.Now().UTC()
	bucketStart := TruncateToBucket(now)
	bucketEnd := bucketStart.Add(bucketInterval)

	tb.logger.Debug("running rollup", "bucket_start", bucketStart, "bucket_end", bucketEnd)

	// Aggregate difficulty per miner for the current bucket.
	rows, err := tb.pool.Query(ctx,
		`SELECT miner_address, SUM(difficulty) AS total_diff
		 FROM p2pool_shares
		 WHERE sidechain = $1
		   AND created_at >= $2
		   AND created_at < $3
		 GROUP BY miner_address`,
		tb.sidechain, bucketStart, bucketEnd)
	if err != nil {
		return fmt.Errorf("querying share difficulty for rollup: %w", err)
	}
	defer rows.Close()

	var upsertCount int
	for rows.Next() {
		var address string
		var totalDiff uint64
		if err := rows.Scan(&address, &totalDiff); err != nil {
			return fmt.Errorf("scanning rollup row: %w", err)
		}

		hashrate := CalculateHashrate(totalDiff)

		_, err := tb.pool.Exec(ctx,
			`INSERT INTO miner_hashrate (miner_address, sidechain, hashrate, bucket_time)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (miner_address, sidechain, bucket_time)
			 DO UPDATE SET hashrate = EXCLUDED.hashrate`,
			address, tb.sidechain, hashrate, bucketStart)
		if err != nil {
			return fmt.Errorf("upserting hashrate for miner %s: %w", address, err)
		}

		// Update Prometheus per-miner gauges.
		metrics.MinerHashrate.WithLabelValues(address, tb.sidechain).Set(float64(hashrate))

		upsertCount++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating rollup rows: %w", err)
	}

	tb.logger.Info("rollup complete", "bucket", bucketStart, "miners_updated", upsertCount)

	// Update MinerShares gauge with current PPLNS window share counts.
	if err := tb.updateShareGauges(ctx); err != nil {
		tb.logger.Error("failed to update share gauges", "err", err)
		// Non-fatal — metrics lag is acceptable.
	}

	return nil
}

// updateShareGauges queries the current share count per miner and updates
// the Prometheus MinerShares gauge. This reflects the PPLNS window share
// count, not all-time totals.
func (tb *TimeseriesBuilder) updateShareGauges(ctx context.Context) error {
	rows, err := tb.pool.Query(ctx,
		`SELECT miner_address, COUNT(*) AS share_count
		 FROM p2pool_shares
		 WHERE sidechain = $1
		   AND created_at > NOW() - INTERVAL '6 hours'
		 GROUP BY miner_address`,
		tb.sidechain)
	if err != nil {
		return fmt.Errorf("querying share counts for gauges: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var address string
		var count int64
		if err := rows.Scan(&address, &count); err != nil {
			return fmt.Errorf("scanning share count row: %w", err)
		}
		metrics.MinerShares.WithLabelValues(address, tb.sidechain).Set(float64(count))
	}
	return rows.Err()
}

// pruneRetention deletes old data for free-tier miners (30-day rolling window).
// Paid-tier miners with extended_retention keep up to 15 months of data.
func (tb *TimeseriesBuilder) pruneRetention(ctx context.Context) error {
	tb.logger.Info("starting retention pruning")

	// Free tier: delete shares, hashrate, and payments older than 30 days
	// for miners who do NOT have extended_retention.
	freeThreshold := time.Now().Add(-30 * 24 * time.Hour)

	tag, err := tb.pool.Exec(ctx,
		`DELETE FROM p2pool_shares
		 WHERE created_at < $1
		   AND miner_address NOT IN (
		     SELECT miner_address FROM subscriptions WHERE extended_retention = TRUE
		   )`,
		freeThreshold)
	if err != nil {
		return fmt.Errorf("pruning free-tier shares: %w", err)
	}
	freeSharesPruned := tag.RowsAffected()

	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM miner_hashrate
		 WHERE bucket_time < $1
		   AND miner_address NOT IN (
		     SELECT miner_address FROM subscriptions WHERE extended_retention = TRUE
		   )`,
		freeThreshold)
	if err != nil {
		return fmt.Errorf("pruning free-tier hashrate: %w", err)
	}
	freeHashratePruned := tag.RowsAffected()

	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM payments
		 WHERE created_at < $1
		   AND miner_address NOT IN (
		     SELECT miner_address FROM subscriptions WHERE extended_retention = TRUE
		   )`,
		freeThreshold)
	if err != nil {
		return fmt.Errorf("pruning free-tier payments: %w", err)
	}
	freePaymentsPruned := tag.RowsAffected()

	// Paid tier: delete data older than 15 months for extended-retention miners.
	paidThreshold := time.Now().Add(-15 * 30 * 24 * time.Hour) // ~15 months

	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM p2pool_shares
		 WHERE created_at < $1
		   AND miner_address IN (
		     SELECT miner_address FROM subscriptions WHERE extended_retention = TRUE
		   )`,
		paidThreshold)
	if err != nil {
		return fmt.Errorf("pruning paid-tier shares: %w", err)
	}
	paidSharesPruned := tag.RowsAffected()

	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM miner_hashrate
		 WHERE bucket_time < $1
		   AND miner_address IN (
		     SELECT miner_address FROM subscriptions WHERE extended_retention = TRUE
		   )`,
		paidThreshold)
	if err != nil {
		return fmt.Errorf("pruning paid-tier hashrate: %w", err)
	}
	paidHashratePruned := tag.RowsAffected()

	tb.logger.Info("retention pruning complete",
		slog.Int64("free_shares_pruned", freeSharesPruned),
		slog.Int64("free_hashrate_pruned", freeHashratePruned),
		slog.Int64("free_payments_pruned", freePaymentsPruned),
		slog.Int64("paid_shares_pruned", paidSharesPruned),
		slog.Int64("paid_hashrate_pruned", paidHashratePruned),
	)

	return nil
}
