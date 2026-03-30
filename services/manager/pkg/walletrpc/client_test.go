package walletrpc

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

func TestCreateAddress(t *testing.T) {
	tests := []struct {
		name       string
		label      string
		response   string
		statusCode int
		wantErr    bool
		check      func(t *testing.T, result *CreateAddressResult)
	}{
		{
			name:  "valid subaddress",
			label: "miner-4abc",
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"result": {
					"address": "8BxKPkFjhLRhcYkq2GXMkVqFPGnoyNdMg9k5b3C6v3B7K1cYfDqwV2LVEb5Nqb7haGP6TUxzhVAfRFS8bRFNmP25ZZ7U2k",
					"address_index": 7
				}
			}`,
			statusCode: http.StatusOK,
			check: func(t *testing.T, result *CreateAddressResult) {
				if result.Address != "8BxKPkFjhLRhcYkq2GXMkVqFPGnoyNdMg9k5b3C6v3B7K1cYfDqwV2LVEb5Nqb7haGP6TUxzhVAfRFS8bRFNmP25ZZ7U2k" {
					t.Errorf("Address mismatch")
				}
				if result.AddressIndex != 7 {
					t.Errorf("AddressIndex = %d, want 7", result.AddressIndex)
				}
			},
		},
		{
			name:       "server error",
			label:      "test",
			response:   `not json`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:  "rpc error",
			label: "test",
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"error": {
					"code": -1,
					"message": "wallet is not open"
				}
			}`,
			statusCode: http.StatusOK,
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

				var req rpcRequest
				body, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(body, &req); err == nil {
					if req.Method != "create_address" {
						t.Errorf("unexpected RPC method: %s", req.Method)
					}
					params, ok := req.Params.(map[string]interface{})
					if ok {
						if label, exists := params["label"]; exists {
							if label != tt.label {
								t.Errorf("label = %v, want %s", label, tt.label)
							}
						}
						if idx, exists := params["account_index"]; exists {
							if idx != float64(0) {
								t.Errorf("account_index = %v, want 0", idx)
							}
						}
					}
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			result, err := client.CreateAddress(context.Background(), tt.label)

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

func TestGetAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		if req.Method != "get_address" {
			t.Errorf("unexpected RPC method: %s", req.Method)
		}

		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "0",
			"result": {
				"address": "4ABC123primaryaddress",
				"addresses": [
					{
						"address": "4ABC123primaryaddress",
						"address_index": 0,
						"label": "Primary",
						"used": true
					},
					{
						"address": "8DEF456subaddress1",
						"address_index": 1,
						"label": "miner-1",
						"used": false
					}
				]
			}
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	result, err := client.GetAddress(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Address != "4ABC123primaryaddress" {
		t.Errorf("primary Address = %s", result.Address)
	}
	if len(result.Addresses) != 2 {
		t.Fatalf("got %d addresses, want 2", len(result.Addresses))
	}
	if result.Addresses[1].AddressIndex != 1 {
		t.Errorf("second address index = %d, want 1", result.Addresses[1].AddressIndex)
	}
	if result.Addresses[1].Label != "miner-1" {
		t.Errorf("second address label = %s, want miner-1", result.Addresses[1].Label)
	}
}

func TestGetTransfers(t *testing.T) {
	tests := []struct {
		name      string
		minHeight uint64
		response  string
		wantErr   bool
		check     func(t *testing.T, result *GetTransfersResult)
	}{
		{
			name:      "incoming transfers",
			minHeight: 3100000,
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"result": {
					"in": [
						{
							"address": "8BxKPksubaddr1",
							"amount": 30000000000,
							"confirmations": 15,
							"height": 3100100,
							"txid": "aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344",
							"subaddr_index": {"major": 0, "minor": 3},
							"timestamp": 1700100000,
							"type": "in",
							"unlock_time": 0
						}
					],
					"pending": [],
					"pool": []
				}
			}`,
			check: func(t *testing.T, result *GetTransfersResult) {
				if len(result.In) != 1 {
					t.Fatalf("got %d incoming transfers, want 1", len(result.In))
				}
				tx := result.In[0]
				if tx.Amount != 30000000000 {
					t.Errorf("Amount = %d, want 30000000000", tx.Amount)
				}
				if tx.Confirmations != 15 {
					t.Errorf("Confirmations = %d, want 15", tx.Confirmations)
				}
				if tx.Height != 3100100 {
					t.Errorf("Height = %d, want 3100100", tx.Height)
				}
				if tx.TxHash != "aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344" {
					t.Errorf("TxHash mismatch")
				}
				if tx.SubaddrIndex.Minor != 3 {
					t.Errorf("SubaddrIndex.Minor = %d, want 3", tx.SubaddrIndex.Minor)
				}
			},
		},
		{
			name:      "empty result",
			minHeight: 0,
			response: `{
				"jsonrpc": "2.0",
				"id": "0",
				"result": {}
			}`,
			check: func(t *testing.T, result *GetTransfersResult) {
				if len(result.In) != 0 {
					t.Errorf("got %d incoming transfers, want 0", len(result.In))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req rpcRequest
				body, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(body, &req); err == nil {
					if req.Method != "get_transfers" {
						t.Errorf("unexpected RPC method: %s", req.Method)
					}
					params, ok := req.Params.(map[string]interface{})
					if ok {
						if in, exists := params["in"]; !exists || in != true {
							t.Error("expected in=true in params")
						}
					}
				}

				_, _ = w.Write([]byte(tt.response))
			}))
			defer srv.Close()

			client := New(srv.URL, testLogger())
			result, err := client.GetTransfers(context.Background(), tt.minHeight)

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

func TestGetHeight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "0",
			"result": {
				"height": 3100500
			}
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	height, err := client.GetHeight(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if height != 3100500 {
		t.Errorf("Height = %d, want 3100500", height)
	}
}

func TestRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": "0",
			"error": {
				"code": -13,
				"message": "No wallet file"
			}
		}`))
	}))
	defer srv.Close()

	client := New(srv.URL, testLogger())
	_, err := client.GetHeight(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify the error message contains the wallet-rpc error
	if got := err.Error(); !contains(got, "wallet-rpc error -13: No wallet file") {
		t.Errorf("error = %q, want it to contain wallet-rpc error", got)
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

	_, err := client.GetHeight(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestRPCErrorString(t *testing.T) {
	e := &RPCError{Code: -13, Message: "No wallet file"}
	want := "wallet-rpc error -13: No wallet file"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
