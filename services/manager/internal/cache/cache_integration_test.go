//go:build integration

package cache_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/cache"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func setupCache(t *testing.T) *cache.Store {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	store, err := cache.New(addr, testLogger())
	if err != nil {
		t.Skipf("skipping cache integration test: redis unavailable: %v", err)
	}
	return store
}

func TestCacheSetGetRoundTrip(t *testing.T) {
	store := setupCache(t)
	defer store.Close()
	ctx := context.Background()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	input := testData{Name: "hashrate", Value: 52000000}
	if err := store.Set(ctx, "test:roundtrip", input, 10*time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var output testData
	found, err := store.Get(ctx, "test:roundtrip", &output)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected key to be found")
	}
	if output.Name != input.Name || output.Value != input.Value {
		t.Errorf("got %+v, want %+v", output, input)
	}

	// Cleanup.
	store.Delete(ctx, "test:roundtrip")
}

func TestCacheGetMiss(t *testing.T) {
	store := setupCache(t)
	defer store.Close()
	ctx := context.Background()

	var output map[string]string
	found, err := store.Get(ctx, "test:nonexistent:"+time.Now().String(), &output)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Error("expected cache miss")
	}
}

func TestCacheDelete(t *testing.T) {
	store := setupCache(t)
	defer store.Close()
	ctx := context.Background()

	store.Set(ctx, "test:delete", "value", 10*time.Second)

	if err := store.Delete(ctx, "test:delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var output string
	found, _ := store.Get(ctx, "test:delete", &output)
	if found {
		t.Error("expected key to be deleted")
	}
}

func TestCacheHealthCheck(t *testing.T) {
	store := setupCache(t)
	defer store.Close()

	if err := store.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	store := setupCache(t)
	defer store.Close()
	ctx := context.Background()

	store.Set(ctx, "test:ttl", "expires", 1*time.Second)

	time.Sleep(1500 * time.Millisecond)

	var output string
	found, _ := store.Get(ctx, "test:ttl", &output)
	if found {
		t.Error("expected key to have expired")
	}
}
