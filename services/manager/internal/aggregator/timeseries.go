package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
// every 15 minutes until the context is cancelled.
func (tb *TimeseriesBuilder) Run(ctx context.Context) error {
	tb.logger.Info("starting timeseries builder", "sidechain", tb.sidechain, "interval", bucketInterval)

	// Run an initial rollup immediately.
	if err := tb.rollup(ctx); err != nil {
		tb.logger.Error("initial rollup failed", "err", err)
	}

	ticker := time.NewTicker(bucketInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			tb.logger.Info("timeseries builder stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := tb.rollup(ctx); err != nil {
				tb.logger.Error("rollup failed", "err", err)
				// Continue running — transient errors should not kill the loop.
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
		upsertCount++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating rollup rows: %w", err)
	}

	tb.logger.Info("rollup complete", "bucket", bucketStart, "miners_updated", upsertCount)
	return nil
}
