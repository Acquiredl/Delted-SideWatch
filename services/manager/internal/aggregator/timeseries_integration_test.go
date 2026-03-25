//go:build integration

package aggregator_test

import (
	"context"
	"testing"
	"time"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/aggregator"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/testhelper"
)

func TestTimeseriesRollupIntegration(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// Insert shares within the current 15-minute bucket.
	bucket := aggregator.TruncateToBucket(time.Now().UTC())
	shareTime := bucket.Add(2 * time.Minute) // inside current bucket

	// Alice: 2 shares with total difficulty 900_000
	// Expected hashrate: 900_000 / 900 = 1000 H/s
	_, err := pool.Exec(ctx,
		`INSERT INTO p2pool_shares (sidechain, miner_address, worker_name, sidechain_height, difficulty, created_at)
		 VALUES ('mini', '4addr_alice', 'rig-01', 7500001, 450000, $1),
		        ('mini', '4addr_alice', 'rig-01', 7500002, 450000, $2)`,
		shareTime, shareTime.Add(30*time.Second))
	if err != nil {
		t.Fatalf("insert shares: %v", err)
	}

	// Bob: 1 share with difficulty 1_800_000
	// Expected hashrate: 1_800_000 / 900 = 2000 H/s
	_, err = pool.Exec(ctx,
		`INSERT INTO p2pool_shares (sidechain, miner_address, worker_name, sidechain_height, difficulty, created_at)
		 VALUES ('mini', '4addr_bob', '', 7500003, 1800000, $1)`,
		shareTime.Add(60*time.Second))
	if err != nil {
		t.Fatalf("insert share: %v", err)
	}

	// Run the timeseries builder.
	tb := aggregator.NewTimeseriesBuilder(pool, "mini", testLogger())
	// We can't call rollup directly since it's unexported, but we can run the builder
	// briefly and check results. Instead, let's just query what the rollup would produce
	// by simulating the query it runs.

	// Actually, let's test Run with a short context that cancels after one cycle.
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = tb.Run(runCtx) // Will run one rollup, then wait, then context cancels.

	// Verify miner_hashrate was populated.
	var aliceHashrate, bobHashrate uint64
	err = pool.QueryRow(ctx,
		`SELECT hashrate FROM miner_hashrate WHERE miner_address = '4addr_alice' AND sidechain = 'mini' AND bucket_time = $1`,
		bucket).Scan(&aliceHashrate)
	if err != nil {
		t.Fatalf("query alice hashrate: %v", err)
	}
	if aliceHashrate != 1000 {
		t.Errorf("alice hashrate = %d, want 1000", aliceHashrate)
	}

	err = pool.QueryRow(ctx,
		`SELECT hashrate FROM miner_hashrate WHERE miner_address = '4addr_bob' AND sidechain = 'mini' AND bucket_time = $1`,
		bucket).Scan(&bobHashrate)
	if err != nil {
		t.Fatalf("query bob hashrate: %v", err)
	}
	if bobHashrate != 2000 {
		t.Errorf("bob hashrate = %d, want 2000", bobHashrate)
	}
}

func TestTimeseriesRollupEmptyBucket(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// No shares — rollup should succeed without inserting anything.
	tb := aggregator.NewTimeseriesBuilder(pool, "mini", testLogger())
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = tb.Run(runCtx)

	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM miner_hashrate`).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 hashrate rows, got %d", count)
	}
}

func TestTimeseriesRollupIdempotent(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	bucket := aggregator.TruncateToBucket(time.Now().UTC())
	shareTime := bucket.Add(1 * time.Minute)

	_, err := pool.Exec(ctx,
		`INSERT INTO p2pool_shares (sidechain, miner_address, worker_name, sidechain_height, difficulty, created_at)
		 VALUES ('mini', '4addr_alice', '', 7500001, 900000, $1)`,
		shareTime)
	if err != nil {
		t.Fatalf("insert share: %v", err)
	}

	// Run rollup twice.
	for i := 0; i < 2; i++ {
		tb := aggregator.NewTimeseriesBuilder(pool, "mini", testLogger())
		runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_ = tb.Run(runCtx)
		cancel()
	}

	// Should still have exactly 1 row (upsert, not duplicate).
	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM miner_hashrate`).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 hashrate row after 2 rollups, got %d", count)
	}
}
