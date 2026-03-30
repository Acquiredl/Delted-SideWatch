package monerod

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestGetLastBlockHeader(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		check      func(t *testing.T, bh *BlockHeader)
	}{
		{
			name: "valid block header",
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"result": {
					"block_header": {
						"height": 3100500,
						"hash": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
						"timestamp": 1700000000,
						"reward": 600000000000,
						"difficulty": 400000000000,
						"num_txes": 15,
						"major_version": 16,
						"nonce": 12345678,
						"orphan_status": false,
						"depth": 0
					}
				}
			}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, bh *BlockHeader) {
				if bh.Height != 3100500 {
					t.Errorf("Height = %d, want 3100500", bh.Height)
				}
				if bh.Hash != "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" {
					t.Errorf("Hash mismatch")
				}
				if bh.Reward != 600000000000 {
					t.Errorf("Reward = %d, want 600000000000", bh.Reward)
				}
				if bh.Difficulty != 400000000000 {
					t.Errorf("Difficulty = %d, want 400000000000", bh.Difficulty)
				}
				if bh.NumTxes != 15 {
					t.Errorf("NumTxes = %d, want 15", bh.NumTxes)
				}
				if bh.MajorVersion != 16 {
					t.Errorf("MajorVersion = %d, want 16", bh.MajorVersion)
				}
				if bh.OrphanStatus {
					t.Error("OrphanStatus = true, want false")
				}
			},
		},
		{
			name:       "server error",
			response:   `not json`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/json_rpc" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				// Verify request body
				var req rpcRequest
				body, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(body, &req); err == nil {
					if req.Method != "get_last_block_header" {
						t.Errorf("unexpected RPC method: %s", req.Method)
					}
					if req.JSONRPC != "2.0" {
						t.Errorf("unexpected JSONRPC version: %s", req.JSONRPC)
					}
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			bh, err := client.GetLastBlockHeader(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, bh)
		})
	}
}

func TestGetBlockHeaderByHeight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		if req.Method != "get_block_header_by_height" {
			t.Errorf("unexpected RPC method: %s", req.Method)
		}

		// Verify params contain height
		params, ok := req.Params.(map[string]interface{})
		if !ok {
			t.Error("expected params to be a map")
		} else if _, exists := params["height"]; !exists {
			t.Error("expected height in params")
		}

		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "0",
			"result": {
				"block_header": {
					"height": 3100000,
					"hash": "1111111111111111111111111111111111111111111111111111111111111111",
					"timestamp": 1699990000,
					"reward": 610000000000,
					"difficulty": 390000000000,
					"num_txes": 10,
					"major_version": 16,
					"nonce": 99999,
					"orphan_status": false,
					"depth": 500
				}
			}
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	bh, err := client.GetBlockHeaderByHeight(context.Background(), 3100000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bh.Height != 3100000 {
		t.Errorf("Height = %d, want 3100000", bh.Height)
	}
	if bh.Depth != 500 {
		t.Errorf("Depth = %d, want 500", bh.Depth)
	}
}

func TestGetBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		if req.Method != "get_block" {
			t.Errorf("unexpected RPC method: %s", req.Method)
		}

		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "0",
			"result": {
				"blob": "0e0eabc...",
				"block_header": {
					"height": 3100500,
					"hash": "aaaa1234",
					"timestamp": 1700000000,
					"reward": 600000000000,
					"difficulty": 400000000000,
					"num_txes": 5,
					"major_version": 16,
					"nonce": 42,
					"orphan_status": false,
					"depth": 0
				},
				"json": "{\"major_version\":16}",
				"miner_tx_hash": "deadbeef1234567890deadbeef1234567890deadbeef1234567890deadbeef12"
			}
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	block, err := client.GetBlock(context.Background(), 3100500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if block.MinerTxHash != "deadbeef1234567890deadbeef1234567890deadbeef1234567890deadbeef12" {
		t.Errorf("MinerTxHash = %s, want deadbeef...", block.MinerTxHash)
	}
	if block.BlockHeader.Height != 3100500 {
		t.Errorf("BlockHeader.Height = %d, want 3100500", block.BlockHeader.Height)
	}
	if block.Blob != "0e0eabc..." {
		t.Errorf("Blob = %s, want 0e0eabc...", block.Blob)
	}
}

func TestGetTransactions(t *testing.T) {
	tests := []struct {
		name       string
		txHashes   []string
		response   string
		statusCode int
		wantErr    bool
		check      func(t *testing.T, result *TransactionResult)
	}{
		{
			name:     "valid transaction",
			txHashes: []string{"deadbeef1234567890deadbeef1234567890deadbeef1234567890deadbeef12"},
			response: `{
				"status": "OK",
				"txs": [
					{
						"tx_hash": "deadbeef1234567890deadbeef1234567890deadbeef1234567890deadbeef12",
						"as_hex": "01...",
						"as_json": "{\"version\":2,\"unlock_time\":3100560,\"vin\":[{\"gen\":{\"height\":3100500}}],\"vout\":[{\"amount\":0,\"target\":{\"tagged_key\":{\"key\":\"aabb\",\"view_tag\":\"cc\"}}}],\"extra\":[1,2,3]}",
						"in_pool": false
					}
				]
			}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, result *TransactionResult) {
				if len(result.Txs) != 1 {
					t.Fatalf("got %d txs, want 1", len(result.Txs))
				}
				tx := result.Txs[0]
				if tx.TxHash != "deadbeef1234567890deadbeef1234567890deadbeef1234567890deadbeef12" {
					t.Errorf("TxHash mismatch")
				}
				if tx.InPool {
					t.Error("InPool = true, want false")
				}

				// Parse the as_json field to verify it's valid
				var txJSON TxJSON
				if err := json.Unmarshal([]byte(tx.AsJSON), &txJSON); err != nil {
					t.Fatalf("failed to parse as_json: %v", err)
				}
				if txJSON.Version != 2 {
					t.Errorf("Version = %d, want 2", txJSON.Version)
				}
				if txJSON.UnlockTime != 3100560 {
					t.Errorf("UnlockTime = %d, want 3100560", txJSON.UnlockTime)
				}
				if len(txJSON.Vin) != 1 || txJSON.Vin[0].Gen == nil {
					t.Fatal("expected coinbase input")
				}
				if txJSON.Vin[0].Gen.Height != 3100500 {
					t.Errorf("Gen.Height = %d, want 3100500", txJSON.Vin[0].Gen.Height)
				}
				if len(txJSON.Vout) != 1 {
					t.Fatalf("got %d outputs, want 1", len(txJSON.Vout))
				}
				if txJSON.Vout[0].Target.TaggedKey == nil {
					t.Fatal("expected tagged_key in output target")
				}
				if txJSON.Vout[0].Target.TaggedKey.Key != "aabb" {
					t.Errorf("TaggedKey.Key = %s, want aabb", txJSON.Vout[0].Target.TaggedKey.Key)
				}
			},
		},
		{
			name:     "missed transactions",
			txHashes: []string{"nonexistent"},
			response: `{
				"status": "OK",
				"txs": [],
				"missed_tx": ["nonexistent"]
			}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, result *TransactionResult) {
				if len(result.Txs) != 0 {
					t.Errorf("got %d txs, want 0", len(result.Txs))
				}
				if len(result.MissedTx) != 1 {
					t.Fatalf("got %d missed txs, want 1", len(result.MissedTx))
				}
				if result.MissedTx[0] != "nonexistent" {
					t.Errorf("MissedTx[0] = %s, want nonexistent", result.MissedTx[0])
				}
			},
		},
		{
			name:       "server error",
			txHashes:   []string{"abc"},
			response:   `error`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/get_transactions" {
					t.Errorf("unexpected path: %s, want /get_transactions", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				// Verify request body
				body, _ := io.ReadAll(r.Body)
				var reqBody getTransactionsRequest
				if err := json.Unmarshal(body, &reqBody); err == nil {
					if !reqBody.DecodeAsJSON {
						t.Error("expected decode_as_json to be true")
					}
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			result, err := client.GetTransactions(context.Background(), tt.txHashes)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, result)
		})
	}
}

func TestRPCError(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  bool
		errMsg   string
	}{
		{
			name: "method not found",
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"error": {
					"code": -32601,
					"message": "Method not found"
				}
			}`,
			wantErr: true,
			errMsg:  "monerod RPC error -32601: Method not found",
		},
		{
			name: "invalid params",
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"error": {
					"code": -32602,
					"message": "Invalid params"
				}
			}`,
			wantErr: true,
			errMsg:  "monerod RPC error -32602: Invalid params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			_, err := client.GetLastBlockHeader(context.Background())

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			// The error message is wrapped, so check it contains the RPC error string
			if got := err.Error(); !contains(got, tt.errMsg) {
				t.Errorf("error = %q, want it to contain %q", got, tt.errMsg)
			}
		})
	}
}

func TestRPCErrorString(t *testing.T) {
	e := &RPCError{Code: -32601, Message: "Method not found"}
	want := "monerod RPC error -32601: Method not found"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
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

	_, err := client.GetLastBlockHeader(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

// contains checks if s contains substr (simple helper to avoid importing strings in test).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
