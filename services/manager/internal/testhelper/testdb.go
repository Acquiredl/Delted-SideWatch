//go:build integration

// Package testhelper provides shared test utilities for integration tests
// that require a real PostgreSQL connection.
package testhelper

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/db"
)

// SetupTestDB connects to a test PostgreSQL instance, runs migrations,
// and truncates all tables. It skips the test if Postgres is unavailable.
// The caller should defer pool.Close().
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	connStr := os.Getenv("TEST_POSTGRES_URL")
	if connStr == "" {
		connStr = "postgres://manager_user:changeme@localhost:5432/p2pool_dashboard?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.New(ctx, connStr)
	if err != nil {
		t.Skipf("skipping integration test: postgres unavailable: %v", err)
	}

	// Run migrations.
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Truncate all tables for a clean state.
	tables := []string{"payments", "miner_hashrate", "p2pool_shares", "p2pool_blocks"}
	for _, table := range tables {
		if _, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)); err != nil {
			pool.Close()
			t.Fatalf("failed to truncate %s: %v", table, err)
		}
	}

	return pool
}
