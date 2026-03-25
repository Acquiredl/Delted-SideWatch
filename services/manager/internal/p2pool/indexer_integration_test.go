//go:build integration

package p2pool_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/p2pool"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/testhelper"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/p2poolclient"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestIndexerIntegration(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	logger := testLogger()

	// Create a mock P2Pool HTTP server.
	now := time.Now()
	mockShares := []p2poolclient.Share{
		{ID: "share-1", Height: 7500001, Difficulty: 300_000_000, Shares: 1, Timestamp: now.Add(-2 * time.Minute).Unix(), MinerAddress: "4addr_alice", WorkerName: "rig-01"},
		{ID: "share-2", Height: 7500002, Difficulty: 200_000_000, Shares: 1, Timestamp: now.Add(-1 * time.Minute).Unix(), MinerAddress: "4addr_bob", WorkerName: ""},
	}
	mockBlocks := []p2poolclient.FoundBlock{
		{MainHeight: 3100000, MainHash: "abcdef1234", SidechainHeight: 7500000, Reward: 615_000_000_000, Timestamp: now.Add(-5 * time.Minute).Unix(), Effort: 92.5},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/shares":
			json.NewEncoder(w).Encode(mockShares)
		case "/api/found_blocks":
			json.NewEncoder(w).Encode(mockBlocks)
		case "/api/pool/stats":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"pool_statistics": map[string]interface{}{
					"hash_rate": 52000000, "miners": 3,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Create the indexer wired to our mock.
	client := p2poolclient.New(srv.URL, logger)
	service := p2pool.NewService(client, "mini", logger)
	indexer := p2pool.NewIndexer(service, pool, 30*time.Second, logger)

	// Run a single indexing cycle by running the indexer with a short timeout.
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = indexer.Run(runCtx)

	// Verify shares were inserted.
	var shareCount int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM p2pool_shares`).Scan(&shareCount)
	if err != nil {
		t.Fatalf("query share count: %v", err)
	}
	if shareCount != 2 {
		t.Errorf("share count = %d, want 2", shareCount)
	}

	// Verify blocks were inserted.
	var blockCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM p2pool_blocks`).Scan(&blockCount)
	if err != nil {
		t.Fatalf("query block count: %v", err)
	}
	if blockCount != 1 {
		t.Errorf("block count = %d, want 1", blockCount)
	}

	// Verify block data.
	var mainHeight int64
	var coinbaseReward int64
	err = pool.QueryRow(ctx, `SELECT main_height, coinbase_reward FROM p2pool_blocks LIMIT 1`).Scan(&mainHeight, &coinbaseReward)
	if err != nil {
		t.Fatalf("query block: %v", err)
	}
	if mainHeight != 3100000 {
		t.Errorf("block main_height = %d, want 3100000", mainHeight)
	}
	if coinbaseReward != 615_000_000_000 {
		t.Errorf("block coinbase_reward = %d, want 615_000_000_000", coinbaseReward)
	}
}

func TestIndexerDeduplication(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	logger := testLogger()

	now := time.Now()
	mockShares := []p2poolclient.Share{
		{ID: "share-1", Height: 7500001, Difficulty: 300_000_000, Shares: 1, Timestamp: now.Unix(), MinerAddress: "4addr_alice"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/shares":
			json.NewEncoder(w).Encode(mockShares)
		case "/api/found_blocks":
			json.NewEncoder(w).Encode([]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := p2poolclient.New(srv.URL, logger)
	service := p2pool.NewService(client, "mini", logger)
	indexer := p2pool.NewIndexer(service, pool, 1*time.Second, logger)

	// Run two indexing cycles with the same data.
	// The first cycle inserts, the second should deduplicate.
	runCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	_ = indexer.Run(runCtx)

	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM p2pool_shares`).Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	// The indexer tracks lastShareHeight, so the same share should only be inserted once.
	if count != 1 {
		t.Errorf("share count after 2 cycles = %d, want 1 (dedup failed)", count)
	}
}
