//go:build integration

package ws_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/aggregator"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/testhelper"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/ws"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestWebSocketHubBroadcast(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	logger := testLogger()

	agg := aggregator.New(pool, "mini", logger)
	hub := ws.NewHub(agg, logger)

	// Start the broadcast loop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Start the WebSocket handler on an httptest server.
	srv := httptest.NewServer(hub.HandlePoolStats())
	defer srv.Close()

	// Connect a WebSocket client.
	wsURL := "ws" + srv.URL[4:] // http -> ws
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Wait for a broadcast message (hub broadcasts every 5s).
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readCancel()

	_, msg, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("WebSocket read: %v", err)
	}

	// Verify it's valid JSON containing pool stats.
	var stats aggregator.PoolOverview
	if err := json.Unmarshal(msg, &stats); err != nil {
		t.Fatalf("unmarshal broadcast: %v\nmessage: %s", err, string(msg))
	}

	// On empty DB, all values should be zero-ish, but the struct should be valid.
	if stats.Sidechain != "mini" {
		t.Errorf("Sidechain = %q, want mini", stats.Sidechain)
	}
}

func TestWebSocketMultipleClients(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	logger := testLogger()

	agg := aggregator.New(pool, "mini", logger)
	hub := ws.NewHub(agg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := httptest.NewServer(hub.HandlePoolStats())
	defer srv.Close()

	wsURL := "ws" + srv.URL[4:]

	// Connect 3 clients.
	conns := make([]*websocket.Conn, 3)
	for i := range conns {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("client %d dial: %v", i, err)
		}
		conns[i] = conn
	}
	defer func() {
		for _, c := range conns {
			c.Close(websocket.StatusNormalClosure, "done")
		}
	}()

	// Each client should receive the broadcast.
	for i, conn := range conns {
		readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
		_, msg, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Fatalf("client %d read: %v", i, err)
		}
		if len(msg) == 0 {
			t.Errorf("client %d received empty message", i)
		}
	}
}
