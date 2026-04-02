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
				"pool_list": ["pplns"],
				"pool_statistics": {
					"hashRate": 23655784,
					"miners": 1000,
					"totalHashes": 2119366504140647,
					"lastBlockFoundTime": 1700000000,
					"lastBlockFound": 3100500,
					"totalBlocksFound": 450,
					"pplnsWeight": 524760289565,
					"pplnsWindowSize": 2160,
					"sidechainDifficulty": 236557843,
					"sidechainHeight": 13401272
				}
			}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, stats *PoolStats) {
				if stats.PoolStatistics.HashRate != 23655784 {
					t.Errorf("HashRate = %d, want 23655784", stats.PoolStatistics.HashRate)
				}
				if stats.PoolStatistics.Miners != 1000 {
					t.Errorf("Miners = %d, want 1000", stats.PoolStatistics.Miners)
				}
				if stats.PoolStatistics.TotalBlocksFound != 450 {
					t.Errorf("TotalBlocksFound = %d, want 450", stats.PoolStatistics.TotalBlocksFound)
				}
				if stats.PoolStatistics.PPLNSWindowSize != 2160 {
					t.Errorf("PPLNSWindowSize = %d, want 2160", stats.PoolStatistics.PPLNSWindowSize)
				}
				if stats.PoolStatistics.SidechainDifficulty != 236557843 {
					t.Errorf("SidechainDifficulty = %d, want 236557843", stats.PoolStatistics.SidechainDifficulty)
				}
				if stats.PoolStatistics.LastBlockFound != 3100500 {
					t.Errorf("LastBlockFound = %d, want 3100500", stats.PoolStatistics.LastBlockFound)
				}
			},
		},
		{
			name:       "empty pool statistics",
			response:   `{"pool_list":[],"pool_statistics": {}}`,
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
				if r.URL.Path != "/pool/stats" {
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

func TestGetLocalStratum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/local/stratum" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Real P2Pool v4.x format: workers are CSV strings
		_, _ = w.Write([]byte(`{
			"hashrate_15m": 15000000,
			"hashrate_1h": 14500000,
			"hashrate_24h": 14000000,
			"total_hashes": 50000000000,
			"total_stratum_shares": 100,
			"shares_found": 100,
			"shares_failed": 2,
			"average_effort": 92.5,
			"current_effort": 75.0,
			"connections": 2,
			"incoming_connections": 0,
			"block_reward_share_percent": 0.002,
			"workers": [
				"192.168.1.10:40000,10000000,30000000000,5000,4AdUndXHHZ6cfufTMvppY6JwXNo",
				"192.168.1.11:40001,5000000,20000000000,3000,48edfHu7V9Z84YzzMa6fUueoELZ"
			]
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	stratum, err := client.GetLocalStratum(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stratum.Hashrate15m != 15000000 {
		t.Errorf("Hashrate15m = %d, want 15000000", stratum.Hashrate15m)
	}
	if stratum.Connections != 2 {
		t.Errorf("Connections = %d, want 2", stratum.Connections)
	}

	workers := stratum.Workers()
	if len(workers) != 2 {
		t.Fatalf("got %d workers, want 2", len(workers))
	}
	if workers[0].Hashrate != 10000000 {
		t.Errorf("Workers[0].Hashrate = %d, want 10000000", workers[0].Hashrate)
	}
	if workers[0].WalletPrefix != "4AdUndXHHZ6cfufTMvppY6JwXNo" {
		t.Errorf("Workers[0].WalletPrefix = %s, want 4AdUndXHHZ6cfufTMvppY6JwXNo", workers[0].WalletPrefix)
	}
	if workers[1].Hashrate != 5000000 {
		t.Errorf("Workers[1].Hashrate = %d, want 5000000", workers[1].Hashrate)
	}
}

func TestParseWorkerCSV(t *testing.T) {
	w := ParseWorkerCSV("70.55.186.208:52804,59,125001,4166,47PoGvdcgHhUn59gTNJbL8JSSZVEHVV")
	if w.Connection != "70.55.186.208:52804" {
		t.Errorf("Connection = %s", w.Connection)
	}
	if w.Hashrate != 59 {
		t.Errorf("Hashrate = %d, want 59", w.Hashrate)
	}
	if w.TotalHashes != 125001 {
		t.Errorf("TotalHashes = %d, want 125001", w.TotalHashes)
	}
	if w.WalletPrefix != "47PoGvdcgHhUn59gTNJbL8JSSZVEHVV" {
		t.Errorf("WalletPrefix = %s", w.WalletPrefix)
	}
}

func TestGetLocalP2P(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/local/p2p" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"connections": 14,
			"incoming_connections": 4,
			"peer_list_size": 397,
			"peers": ["O,248,125,P2Pool v4.13,13401261,65.21.227.114:37888", "I,11,223,P2Pool v4.13,13401261,88.146.114.222:33290"]
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	p2p, err := client.GetLocalP2P(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p2p.Connections != 14 {
		t.Errorf("Connections = %d, want 14", p2p.Connections)
	}
	if p2p.PeerListSize != 397 {
		t.Errorf("PeerListSize = %d, want 397", p2p.PeerListSize)
	}
	if len(p2p.Peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(p2p.Peers))
	}
}

func TestGetNetworkStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/network/stats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"difficulty": 603438092168,
			"hash": "abcdef1234",
			"height": 3643738,
			"reward": 606328980000,
			"timestamp": 1775146302
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	stats, err := client.GetNetworkStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Height != 3643738 {
		t.Errorf("Height = %d, want 3643738", stats.Height)
	}
	if stats.Difficulty != 603438092168 {
		t.Errorf("Difficulty = %d, want 603438092168", stats.Difficulty)
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetPoolStats(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
