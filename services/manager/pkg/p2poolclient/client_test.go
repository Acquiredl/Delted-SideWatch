package p2poolclient

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestGetPoolStats(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		check      func(t *testing.T, stats *PoolStats)
	}{
		{
			name: "valid response",
			response: `{
				"pool_statistics": {
					"hash_rate": 52000000,
					"hash_rate_short": 48000000,
					"miners": 1234,
					"total_hashes": 999999999,
					"last_block_found": 3100000,
					"totalBlocksFound": 450,
					"pplns_window": 2160,
					"sidechainDifficulty": 300000000,
					"sidechainHeight": 7500000
				}
			}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, stats *PoolStats) {
				if stats.PoolStatistics.HashRate != 52000000 {
					t.Errorf("HashRate = %d, want 52000000", stats.PoolStatistics.HashRate)
				}
				if stats.PoolStatistics.HashRateShort != 48000000 {
					t.Errorf("HashRateShort = %d, want 48000000", stats.PoolStatistics.HashRateShort)
				}
				if stats.PoolStatistics.Miners != 1234 {
					t.Errorf("Miners = %d, want 1234", stats.PoolStatistics.Miners)
				}
				if stats.PoolStatistics.TotalBlocks != 450 {
					t.Errorf("TotalBlocks = %d, want 450", stats.PoolStatistics.TotalBlocks)
				}
				if stats.PoolStatistics.PPLNSWindow != 2160 {
					t.Errorf("PPLNSWindow = %d, want 2160", stats.PoolStatistics.PPLNSWindow)
				}
				if stats.PoolStatistics.SidechainDifficulty != 300000000 {
					t.Errorf("SidechainDifficulty = %d, want 300000000", stats.PoolStatistics.SidechainDifficulty)
				}
				if stats.PoolStatistics.SidechainHeight != 7500000 {
					t.Errorf("SidechainHeight = %d, want 7500000", stats.PoolStatistics.SidechainHeight)
				}
			},
		},
		{
			name:       "empty pool statistics",
			response:   `{"pool_statistics": {}}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, stats *PoolStats) {
				if stats.PoolStatistics.Miners != 0 {
					t.Errorf("Miners = %d, want 0", stats.PoolStatistics.Miners)
				}
			},
		},
		{
			name:       "server error",
			response:   `Internal Server Error`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/pool/stats" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			stats, err := client.GetPoolStats(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, stats)
		})
	}
}

func TestGetFoundBlocks(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		wantLen    int
		check      func(t *testing.T, blocks []FoundBlock)
	}{
		{
			name: "multiple blocks",
			response: `[
				{
					"height": 3100500,
					"hash": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					"sidechain_height": 7500100,
					"reward": 600000000000,
					"timestamp": 1700000000,
					"effort": 85.5
				},
				{
					"height": 3100400,
					"hash": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					"sidechain_height": 7499900,
					"reward": 610000000000,
					"timestamp": 1699999000,
					"effort": 120.3
				}
			]`,
			statusCode: http.StatusOK,
			wantLen:    2,
			check: func(t *testing.T, blocks []FoundBlock) {
				if blocks[0].MainHeight != 3100500 {
					t.Errorf("blocks[0].MainHeight = %d, want 3100500", blocks[0].MainHeight)
				}
				if blocks[0].MainHash != "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" {
					t.Errorf("blocks[0].MainHash = %s, want abcdef...", blocks[0].MainHash)
				}
				if blocks[0].Reward != 600000000000 {
					t.Errorf("blocks[0].Reward = %d, want 600000000000", blocks[0].Reward)
				}
				if blocks[0].Effort != 85.5 {
					t.Errorf("blocks[0].Effort = %f, want 85.5", blocks[0].Effort)
				}
				if blocks[1].SidechainHeight != 7499900 {
					t.Errorf("blocks[1].SidechainHeight = %d, want 7499900", blocks[1].SidechainHeight)
				}
			},
		},
		{
			name:       "empty array",
			response:   `[]`,
			statusCode: http.StatusOK,
			wantLen:    0,
			check:      func(t *testing.T, blocks []FoundBlock) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/found_blocks" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			blocks, err := client.GetFoundBlocks(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(blocks) != tt.wantLen {
				t.Fatalf("got %d blocks, want %d", len(blocks), tt.wantLen)
			}
			tt.check(t, blocks)
		})
	}
}

func TestGetShares(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		wantLen    int
		check      func(t *testing.T, shares []Share)
	}{
		{
			name: "valid shares",
			response: `[
				{
					"id": "share-001",
					"height": 7500050,
					"difficulty": 300000000,
					"shares": 1,
					"timestamp": 1700000100,
					"address": "4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq",
					"worker": "rig-01"
				},
				{
					"id": "share-002",
					"height": 7500051,
					"difficulty": 300000000,
					"shares": 1,
					"timestamp": 1700000130,
					"address": "48edfHu7V9Z84YzzMa6fUueoELZ9ZRXq9VetWzYGzKt52XU5xvqgzYnDK9URnRhJP1UdtKNkStMwk9qKBFdvQZCP9tkBavH",
					"worker": ""
				}
			]`,
			statusCode: http.StatusOK,
			wantLen:    2,
			check: func(t *testing.T, shares []Share) {
				if shares[0].ID != "share-001" {
					t.Errorf("shares[0].ID = %s, want share-001", shares[0].ID)
				}
				if shares[0].MinerAddress != "4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq" {
					t.Errorf("shares[0].MinerAddress mismatch")
				}
				if shares[0].WorkerName != "rig-01" {
					t.Errorf("shares[0].WorkerName = %s, want rig-01", shares[0].WorkerName)
				}
				// Test empty optional field
				if shares[1].WorkerName != "" {
					t.Errorf("shares[1].WorkerName = %s, want empty", shares[1].WorkerName)
				}
				if shares[1].Height != 7500051 {
					t.Errorf("shares[1].Height = %d, want 7500051", shares[1].Height)
				}
			},
		},
		{
			name:       "empty array",
			response:   `[]`,
			statusCode: http.StatusOK,
			wantLen:    0,
			check:      func(t *testing.T, shares []Share) {},
		},
		{
			name:       "missing optional fields",
			response:   `[{"id": "share-003", "height": 100, "difficulty": 50000, "shares": 1, "timestamp": 1700000000, "address": "4Addr"}]`,
			statusCode: http.StatusOK,
			wantLen:    1,
			check: func(t *testing.T, shares []Share) {
				if shares[0].WorkerName != "" {
					t.Errorf("expected empty worker name for missing field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/shares" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			shares, err := client.GetShares(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(shares) != tt.wantLen {
				t.Fatalf("got %d shares, want %d", len(shares), tt.wantLen)
			}
			tt.check(t, shares)
		})
	}
}

func TestGetWorkerStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/worker_stats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq": {
				"shares": 42,
				"hashes": 12600000000,
				"last_share": 1700000100
			}
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	stats, err := client.GetWorkerStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addr := "4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq"
	info, ok := stats[addr]
	if !ok {
		t.Fatalf("expected address %s in stats", addr)
	}
	if info.Shares != 42 {
		t.Errorf("Shares = %d, want 42", info.Shares)
	}
	if info.Hashes != 12600000000 {
		t.Errorf("Hashes = %d, want 12600000000", info.Hashes)
	}
	if info.LastShare != 1700000100 {
		t.Errorf("LastShare = %d, want 1700000100", info.LastShare)
	}
}

func TestGetPeers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/p2p/peers" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"id": "peer-1", "addr": "192.168.1.10:37889"},
			{"id": "peer-2", "addr": "10.0.0.5:37889"}
		]`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	peers, err := client.GetPeers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(peers))
	}
	if peers[0].ID != "peer-1" {
		t.Errorf("peers[0].ID = %s, want peer-1", peers[0].ID)
	}
	if peers[1].Addr != "10.0.0.5:37889" {
		t.Errorf("peers[1].Addr = %s, want 10.0.0.5:37889", peers[1].Addr)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response — the context should cancel before we respond
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetPoolStats(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
