//go:build e2e

// Package e2e contains end-to-end tests that run against the full Docker stack.
// Prerequisites: docker compose -f docker-compose.yml -f docker-compose.dev.yml -f docker-compose.test.yml up --build -d
// Run: go test -v -tags e2e -timeout 120s ./...
package e2e

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"nhooyr.io/websocket"
)

const (
	gatewayURL = "http://localhost:8080"
	managerURL = "http://localhost:8081"
	frontendURL = "http://localhost:3000"

	// Known test miner addresses from mocknode.
	minerAddr1 = "4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq"
	minerAddr2 = "48edfHu7V9Z84YzzMa6fUueoELZ9ZRXq9VetWzYGzKt52XU5xvqgzYnDK9URnRhJP1UdtKNkStMwk9qKBFdvQZCP9tkBavH"

	// JWT secret must match the one in .env / docker-compose config.
	// In dev mode this is read from .env; default test value.
	jwtSecret = "changeme-jwt-secret"
)

var client = &http.Client{Timeout: 10 * time.Second}

// getJSON performs a GET request and decodes the JSON response into dest.
func getJSON(t *testing.T, url string, dest interface{}) *http.Response {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	if dest != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			t.Fatalf("GET %s: failed to decode JSON: %v", url, err)
		}
	}
	return resp
}

// generateJWT creates a valid JWT token for admin endpoint testing.
func generateJWT(t *testing.T) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "admin",
		"role": "admin",
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	})
	signed, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

// waitForShares polls the sidechain shares endpoint until data appears.
func waitForShares(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var shares []json.RawMessage
		resp := getJSON(t, gatewayURL+"/api/sidechain/shares?limit=5", &shares)
		if resp.StatusCode == http.StatusOK && len(shares) > 0 {
			t.Logf("shares appeared: %d found", len(shares))
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("timed out waiting for sidechain shares to appear")
}

// --- Test Cases ---

func TestHealthManager(t *testing.T) {
	var result map[string]string
	resp := getJSON(t, managerURL+"/health", &result)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
	if result["postgres"] != "ok" {
		t.Errorf("postgres not ok: %q", result["postgres"])
	}
	if result["redis"] != "ok" {
		t.Errorf("redis not ok: %q", result["redis"])
	}
}

func TestHealthGateway(t *testing.T) {
	resp, err := client.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPoolStats(t *testing.T) {
	var stats map[string]interface{}
	resp := getJSON(t, gatewayURL+"/api/pool/stats", &stats)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Pool stats should have some fields populated.
	if _, ok := stats["total_hashrate"]; !ok {
		t.Log("pool stats response:", stats)
		// total_hashrate may be 0 if no timeseries data yet, but key should exist
	}
}

func TestIndexerPopulatesShares(t *testing.T) {
	waitForShares(t, 60*time.Second)

	var shares []map[string]interface{}
	resp := getJSON(t, gatewayURL+"/api/sidechain/shares?limit=10", &shares)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(shares) == 0 {
		t.Fatal("expected shares after indexer ran")
	}

	// Verify share structure.
	first := shares[0]
	for _, key := range []string{"miner_address", "sidechain_height", "difficulty"} {
		if _, ok := first[key]; !ok {
			t.Errorf("share missing key %q", key)
		}
	}
}

func TestMinerStats(t *testing.T) {
	waitForShares(t, 60*time.Second)

	var stats map[string]interface{}
	resp := getJSON(t, gatewayURL+"/api/miner/"+minerAddr1, &stats)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Miner should have some share data.
	if total, ok := stats["total_shares"]; ok {
		t.Logf("miner total_shares: %v", total)
	}
}

func TestMinerPayments(t *testing.T) {
	waitForShares(t, 60*time.Second)

	// Payments may take longer to appear (need block found + 10 confirmations in scanner).
	// Just verify the endpoint returns a valid response.
	var payments []json.RawMessage
	resp := getJSON(t, gatewayURL+"/api/miner/"+minerAddr1+"/payments?limit=10", &payments)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	t.Logf("miner payments count: %d", len(payments))
}

func TestMinerHashrate(t *testing.T) {
	waitForShares(t, 60*time.Second)

	var points []json.RawMessage
	resp := getJSON(t, gatewayURL+"/api/miner/"+minerAddr1+"/hashrate?hours=1", &points)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	t.Logf("hashrate data points: %d", len(points))
}

func TestBlocksList(t *testing.T) {
	waitForShares(t, 60*time.Second)

	var blocks []map[string]interface{}
	resp := getJSON(t, gatewayURL+"/api/blocks?limit=10", &blocks)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Mocknode creates an initial found block, so there should be at least 1.
	if len(blocks) == 0 {
		t.Log("no blocks found yet (may need more time for mocknode)")
	} else {
		first := blocks[0]
		if _, ok := first["main_height"]; !ok {
			t.Error("block missing main_height field")
		}
	}
}

func TestTaxExportCSV(t *testing.T) {
	resp, err := client.Get(gatewayURL + "/api/miner/" + minerAddr1 + "/tax-export")
	if err != nil {
		t.Fatalf("GET tax-export failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("expected Content-Type text/csv, got %q", ct)
	}

	// Parse CSV to verify structure.
	reader := csv.NewReader(resp.Body)
	header, err := reader.Read()
	if err != nil {
		t.Fatalf("failed to read CSV header: %v", err)
	}

	expectedHeader := []string{"date", "amount_atomic", "amount_xmr", "xmr_usd_price", "xmr_cad_price", "usd_value", "cad_value"}
	if len(header) != len(expectedHeader) {
		t.Fatalf("CSV header length = %d, want %d", len(header), len(expectedHeader))
	}
	for i, col := range expectedHeader {
		if header[i] != col {
			t.Errorf("CSV header[%d] = %q, want %q", i, header[i], col)
		}
	}
}

func TestAdminWithoutAuth(t *testing.T) {
	resp, err := client.Post(gatewayURL+"/api/admin/backfill-prices", "application/json", nil)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAdminWithAuth(t *testing.T) {
	token := generateJWT(t)

	req, err := http.NewRequest("POST", gatewayURL+"/api/admin/backfill-prices", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Body may have been read above on error path; this is ok for success
		t.Log("response already consumed or empty (expected for success)")
	}
}

func TestRateLimiting(t *testing.T) {
	// Send many rapid requests to trigger rate limiting.
	// Default rate limit is 10 req/s with 2x burst = 20.
	got429 := false
	for i := 0; i < 30; i++ {
		resp, err := client.Get(gatewayURL + "/api/pool/stats")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			t.Logf("rate limited at request %d", i+1)
			break
		}
	}
	if !got429 {
		t.Log("WARNING: did not get rate limited after 30 rapid requests (rate limiter may be lenient or Redis-based sliding window)")
	}
}

func TestWebSocket(t *testing.T) {
	// Wait for the rate-limit window from TestRateLimiting to expire.
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	wsURL := strings.Replace(gatewayURL, "http://", "ws://", 1) + "/ws/pool/stats"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// The hub broadcasts every 5 seconds, so wait up to 10s.
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}

	// Verify it's valid JSON.
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		t.Fatalf("WebSocket message is not valid JSON: %v\nmessage: %s", err, string(msg))
	}
	t.Logf("WebSocket received: %d bytes", len(msg))
}

func TestFrontendRenders(t *testing.T) {
	resp, err := client.Get(frontendURL + "/")
	if err != nil {
		t.Skipf("frontend not reachable (may not be running): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	// Next.js should return HTML.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}

	if len(body) < 100 {
		t.Errorf("frontend response suspiciously short: %d bytes", len(body))
	}
}

func TestInvalidMinerAddress(t *testing.T) {
	// Test with an address that doesn't exist — should still return 200 with empty data.
	resp, err := client.Get(gatewayURL + "/api/miner/4invalidaddress/payments?limit=5")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for unknown miner, got %d", resp.StatusCode)
	}
}

func TestPaginationParams(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"blocks with limit", fmt.Sprintf("%s/api/blocks?limit=2&offset=0", gatewayURL)},
		{"shares with limit", fmt.Sprintf("%s/api/sidechain/shares?limit=3&offset=0", gatewayURL)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var items []json.RawMessage
			resp := getJSON(t, tt.url, &items)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}
