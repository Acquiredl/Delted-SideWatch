//go:build integration

package aggregator_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/aggregator"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/testhelper"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func seedShares(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	now := time.Now()
	shares := []struct {
		sidechain string
		address   string
		worker    string
		height    int64
		diff      int64
		createdAt time.Time
	}{
		{"mini", "4addr_alice", "rig-01", 7500001, 300_000_000, now.Add(-5 * time.Minute)},
		{"mini", "4addr_alice", "rig-01", 7500002, 300_000_000, now.Add(-3 * time.Minute)},
		{"mini", "4addr_bob", "", 7500003, 200_000_000, now.Add(-1 * time.Minute)},
	}
	for _, s := range shares {
		_, err := pool.Exec(ctx,
			`INSERT INTO p2pool_shares (sidechain, miner_address, worker_name, sidechain_height, difficulty, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			s.sidechain, s.address, s.worker, s.height, s.diff, s.createdAt)
		if err != nil {
			t.Fatalf("seed share: %v", err)
		}
	}
}

func seedBlocks(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO p2pool_blocks (main_height, main_hash, sidechain_height, coinbase_reward, effort, found_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		3100000, "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		7500000, 1_000_000_000_000, 95.5, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("seed block: %v", err)
	}
}

func seedPayments(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	payments := []struct {
		address string
		amount  int64
		height  int64
		hash    string
	}{
		{"4addr_alice", 600_000_000_000, 3100000, "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
		{"4addr_bob", 400_000_000_000, 3100000, "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
	}
	for _, p := range payments {
		_, err := pool.Exec(ctx,
			`INSERT INTO payments (miner_address, amount, main_height, main_hash) VALUES ($1, $2, $3, $4)`,
			p.address, p.amount, p.height, p.hash)
		if err != nil {
			t.Fatalf("seed payment: %v", err)
		}
	}
}

func seedHashrate(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	bucket := aggregator.TruncateToBucket(time.Now().UTC())
	entries := []struct {
		address   string
		sidechain string
		hashrate  int64
	}{
		{"4addr_alice", "mini", 333_333},
		{"4addr_bob", "mini", 222_222},
	}
	for _, e := range entries {
		_, err := pool.Exec(ctx,
			`INSERT INTO miner_hashrate (miner_address, sidechain, hashrate, bucket_time) VALUES ($1, $2, $3, $4)`,
			e.address, e.sidechain, e.hashrate, bucket)
		if err != nil {
			t.Fatalf("seed hashrate: %v", err)
		}
	}
}

func TestAggregatorGetPoolStats(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedShares(t, pool, ctx)
	seedBlocks(t, pool, ctx)
	seedPayments(t, pool, ctx)
	seedHashrate(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	stats, err := agg.GetPoolStats(ctx)
	if err != nil {
		t.Fatalf("GetPoolStats: %v", err)
	}

	if stats.TotalMiners != 2 {
		t.Errorf("TotalMiners = %d, want 2", stats.TotalMiners)
	}
	if stats.BlocksFound != 1 {
		t.Errorf("BlocksFound = %d, want 1", stats.BlocksFound)
	}
	if stats.TotalPaid != 1_000_000_000_000 {
		t.Errorf("TotalPaid = %d, want 1_000_000_000_000", stats.TotalPaid)
	}
	if stats.TotalHashrate != 333_333+222_222 {
		t.Errorf("TotalHashrate = %d, want %d", stats.TotalHashrate, 333_333+222_222)
	}
	if stats.Sidechain != "mini" {
		t.Errorf("Sidechain = %q, want mini", stats.Sidechain)
	}
}

func TestAggregatorGetMinerStats(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedShares(t, pool, ctx)
	seedPayments(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	stats, err := agg.GetMinerStats(ctx, "4addr_alice")
	if err != nil {
		t.Fatalf("GetMinerStats: %v", err)
	}

	if stats.Address != "4addr_alice" {
		t.Errorf("Address = %q, want 4addr_alice", stats.Address)
	}
	if stats.TotalShares != 2 {
		t.Errorf("TotalShares = %d, want 2", stats.TotalShares)
	}
	if stats.TotalPaid != 600_000_000_000 {
		t.Errorf("TotalPaid = %d, want 600_000_000_000", stats.TotalPaid)
	}
}

func TestAggregatorGetMinerPayments(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedPayments(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	payments, err := agg.GetMinerPayments(ctx, "4addr_alice", 50, 0, 0)
	if err != nil {
		t.Fatalf("GetMinerPayments: %v", err)
	}

	if len(payments) != 1 {
		t.Fatalf("got %d payments, want 1", len(payments))
	}
	if payments[0].Amount != 600_000_000_000 {
		t.Errorf("Amount = %d, want 600_000_000_000", payments[0].Amount)
	}
	if payments[0].MainHeight != 3100000 {
		t.Errorf("MainHeight = %d, want 3100000", payments[0].MainHeight)
	}
}

func TestAggregatorGetMinerHashrate(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedHashrate(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	points, err := agg.GetMinerHashrate(ctx, "4addr_alice", 1)
	if err != nil {
		t.Fatalf("GetMinerHashrate: %v", err)
	}

	if len(points) != 1 {
		t.Fatalf("got %d points, want 1", len(points))
	}
	if points[0].Hashrate != 333_333 {
		t.Errorf("Hashrate = %d, want 333_333", points[0].Hashrate)
	}
}

func TestAggregatorGetBlocks(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedBlocks(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	blocks, err := agg.GetBlocks(ctx, 50, 0)
	if err != nil {
		t.Fatalf("GetBlocks: %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if blocks[0].MainHeight != 3100000 {
		t.Errorf("MainHeight = %d, want 3100000", blocks[0].MainHeight)
	}
	if blocks[0].CoinbaseReward != 1_000_000_000_000 {
		t.Errorf("CoinbaseReward = %d, want 1_000_000_000_000", blocks[0].CoinbaseReward)
	}
}

func TestAggregatorGetSidechainShares(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedShares(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	shares, err := agg.GetSidechainShares(ctx, 100, 0)
	if err != nil {
		t.Fatalf("GetSidechainShares: %v", err)
	}

	if len(shares) != 3 {
		t.Fatalf("got %d shares, want 3", len(shares))
	}
	// Shares should be ordered by created_at DESC.
	if shares[0].CreatedAt.Before(shares[len(shares)-1].CreatedAt) {
		t.Error("shares not in descending order")
	}
}

func TestAggregatorGetMinerPaymentsForExport(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedPayments(t, pool, ctx)

	agg := aggregator.New(pool, nil, "mini", testLogger())
	payments, err := agg.GetMinerPaymentsForExport(ctx, "4addr_alice")
	if err != nil {
		t.Fatalf("GetMinerPaymentsForExport: %v", err)
	}

	if len(payments) != 1 {
		t.Fatalf("got %d payments, want 1", len(payments))
	}
}

func TestAggregatorEmptyResults(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	agg := aggregator.New(pool, nil, "mini", testLogger())

	stats, err := agg.GetPoolStats(ctx)
	if err != nil {
		t.Fatalf("GetPoolStats on empty DB: %v", err)
	}
	if stats.TotalMiners != 0 {
		t.Errorf("expected 0 miners, got %d", stats.TotalMiners)
	}

	minerStats, err := agg.GetMinerStats(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetMinerStats on empty DB: %v", err)
	}
	if minerStats.TotalShares != 0 {
		t.Errorf("expected 0 shares, got %d", minerStats.TotalShares)
	}

	blocks, err := agg.GetBlocks(ctx, 50, 0)
	if err != nil {
		t.Fatalf("GetBlocks on empty DB: %v", err)
	}
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}
