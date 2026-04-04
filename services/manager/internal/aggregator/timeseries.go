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

// pruneRetention deletes old data based on subscription status.
//
// Phases:
//  1. Nuke — immediately delete ALL data for miners who requested deletion.
//  2. Held-back setup — for lapsed subscribers with previous-year payment data,
//     preserve that year for tax export (2 downloads).
//  3. Held-back cleanup — clear held status for miners who used all exports.
//  4. Free/lapsed — prune shares + hashrate at 30 days.
//  5. Free/lapsed — prune payments at 30 days, excluding held-year payments.
//  6. Active paid — prune everything at 15 months.
func (tb *TimeseriesBuilder) pruneRetention(ctx context.Context) error {
	tb.logger.Info("starting retention pruning")
	now := time.Now().UTC()
	previousYear := now.Year() - 1

	// Phase 1: Nuke — delete ALL data for miners who requested full deletion.
	nuked, err := tb.pruneNuked(ctx)
	if err != nil {
		return fmt.Errorf("pruning nuked miners: %w", err)
	}

	// Phase 2: Set up held-back year for newly-lapsed subscribers.
	// When a subscriber's grace period ends and they have payment data from
	// the previous calendar year, preserve it for tax export (2 downloads).
	tag, err := tb.pool.Exec(ctx,
		`UPDATE subscriptions
		 SET held_year = $1, tax_exports_remaining = 2
		 WHERE grace_until IS NOT NULL AND grace_until <= NOW()
		   AND held_year IS NULL
		   AND data_deleted_at IS NULL
		   AND miner_address IN (
		     SELECT DISTINCT miner_address FROM payments
		     WHERE EXTRACT(YEAR FROM created_at) = $1
		   )`,
		previousYear)
	if err != nil {
		return fmt.Errorf("setting up held-back year: %w", err)
	}
	heldBackSetup := tag.RowsAffected()

	// Phase 3: Clear held-back status for miners who used all their exports.
	tag, err = tb.pool.Exec(ctx,
		`UPDATE subscriptions
		 SET held_year = NULL, tax_exports_remaining = NULL
		 WHERE tax_exports_remaining IS NOT NULL AND tax_exports_remaining <= 0`)
	if err != nil {
		return fmt.Errorf("clearing exhausted held-back data: %w", err)
	}
	heldBackCleared := tag.RowsAffected()

	// Phase 4: Free/lapsed — prune shares and hashrate older than 30 days.
	freeThreshold := now.Add(-30 * 24 * time.Hour)

	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM p2pool_shares
		 WHERE created_at < $1
		   AND miner_address NOT IN (
		     SELECT miner_address FROM subscriptions WHERE grace_until > NOW()
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
		     SELECT miner_address FROM subscriptions WHERE grace_until > NOW()
		   )`,
		freeThreshold)
	if err != nil {
		return fmt.Errorf("pruning free-tier hashrate: %w", err)
	}
	freeHashratePruned := tag.RowsAffected()

	// Phase 5: Free/lapsed — prune payments older than 30 days, but preserve
	// held-year payment data for miners who still have tax exports remaining.
	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM payments p
		 WHERE p.created_at < $1
		   AND p.miner_address NOT IN (
		     SELECT miner_address FROM subscriptions WHERE grace_until > NOW()
		   )
		   AND NOT EXISTS (
		     SELECT 1 FROM subscriptions s
		     WHERE s.miner_address = p.miner_address
		       AND s.held_year IS NOT NULL
		       AND s.tax_exports_remaining > 0
		       AND EXTRACT(YEAR FROM p.created_at) = s.held_year
		   )`,
		freeThreshold)
	if err != nil {
		return fmt.Errorf("pruning free-tier payments: %w", err)
	}
	freePaymentsPruned := tag.RowsAffected()

	// Phase 6: Active paid — prune everything older than 15 months.
	paidThreshold := now.Add(-15 * 30 * 24 * time.Hour) // ~15 months

	tag, err = tb.pool.Exec(ctx,
		`DELETE FROM p2pool_shares
		 WHERE created_at < $1
		   AND miner_address IN (
		     SELECT miner_address FROM subscriptions WHERE grace_until > NOW()
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
		     SELECT miner_address FROM subscriptions WHERE grace_until > NOW()
		   )`,
		paidThreshold)
	if err != nil {
		return fmt.Errorf("pruning paid-tier hashrate: %w", err)
	}
	paidHashratePruned := tag.RowsAffected()

	tb.logger.Info("retention pruning complete",
		slog.Int64("nuked_miners", nuked),
		slog.Int64("held_back_setup", heldBackSetup),
		slog.Int64("held_back_cleared", heldBackCleared),
		slog.Int64("free_shares_pruned", freeSharesPruned),
		slog.Int64("free_hashrate_pruned", freeHashratePruned),
		slog.Int64("free_payments_pruned", freePaymentsPruned),
		slog.Int64("paid_shares_pruned", paidSharesPruned),
		slog.Int64("paid_hashrate_pruned", paidHashratePruned),
	)

	return nil
}

// pruneNuked deletes ALL data for miners who requested full data deletion.
// Returns the number of miners whose data was purged.
func (tb *TimeseriesBuilder) pruneNuked(ctx context.Context) (int64, error) {
	// Find miners pending deletion.
	rows, err := tb.pool.Query(ctx,
		`SELECT miner_address FROM subscriptions WHERE data_deleted_at IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("querying nuked miners: %w", err)
	}
	defer rows.Close()

	var addresses []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return 0, fmt.Errorf("scanning nuked miner address: %w", err)
		}
		addresses = append(addresses, addr)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating nuked miner rows: %w", err)
	}

	if len(addresses) == 0 {
		return 0, nil
	}

	for _, addr := range addresses {
		// Delete all miner data from every table.
		for _, query := range []string{
			`DELETE FROM p2pool_shares WHERE miner_address = $1`,
			`DELETE FROM miner_hashrate WHERE miner_address = $1`,
			`DELETE FROM payments WHERE miner_address = $1`,
			`DELETE FROM subscription_payments WHERE miner_address = $1`,
		} {
			if _, err := tb.pool.Exec(ctx, query, addr); err != nil {
				return 0, fmt.Errorf("nuking data for %s: %w", addr, err)
			}
		}

		// Reset subscription to free, clear all flags, keep the row for audit.
		_, err := tb.pool.Exec(ctx,
			`UPDATE subscriptions
			 SET tier = 'free', expires_at = NULL, grace_until = NULL,
			     api_key_hash = NULL, held_year = NULL, tax_exports_remaining = NULL,
			     extended_retention = FALSE, retention_since = NULL,
			     updated_at = NOW()
			 WHERE miner_address = $1`,
			addr)
		if err != nil {
			return 0, fmt.Errorf("resetting subscription for nuked miner %s: %w", addr, err)
		}

		tb.logger.Info("miner data nuked", slog.String("miner_address", addr))
	}

	return int64(len(addresses)), nil
}
