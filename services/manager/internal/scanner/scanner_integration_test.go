//go:build integration

package scanner_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/scanner"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/testhelper"
)

func seedTestBlock(t *testing.T, pool *pgxpool.Pool, ctx context.Context, height int64, reward int64) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO p2pool_blocks (main_height, main_hash, sidechain_height, coinbase_reward, effort, found_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		height, "testhash1234567890testhash1234567890testhash1234567890testhash12",
		7500000, reward, 95.0, time.Now().Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("seed block: %v", err)
	}
}

func seedTestShares(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()
	shares := []struct {
		address string
		diff    int64
	}{
		{"4addr_alice", 600},
		{"4addr_alice", 600},
		{"4addr_bob", 400},
		{"4addr_bob", 400},
	}
	for i, s := range shares {
		_, err := pool.Exec(ctx,
			`INSERT INTO p2pool_shares (sidechain, miner_address, worker_name, sidechain_height, difficulty, created_at)
			 VALUES ('mini', $1, '', $2, $3, $4)`,
			s.address, 7500000+int64(i), s.diff, time.Now().Add(-time.Duration(10-i)*time.Minute))
		if err != nil {
			t.Fatalf("seed share: %v", err)
		}
	}
}

func TestExtractPaymentsIntegration(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	// Seed: block with 1 XMR reward, and shares for alice (60%) and bob (40%).
	seedTestBlock(t, pool, ctx, 3100000, 1_000_000_000_000)
	seedTestShares(t, pool, ctx)

	// ExtractPayments reads from p2pool_blocks and p2pool_shares.
	// The txJSON parameter is not used for proportional calculation,
	// but we still need to pass a valid one.
	payments, err := scanner.ExtractPayments(ctx, pool, nil, 3100000, "testhash1234567890testhash1234567890testhash1234567890testhash12")
	if err != nil {
		t.Fatalf("ExtractPayments: %v", err)
	}

	if len(payments) != 2 {
		t.Fatalf("got %d payments, want 2", len(payments))
	}

	// Build payment map for order-independent checking.
	paymentMap := make(map[string]uint64)
	var total uint64
	for _, p := range payments {
		paymentMap[p.MinerAddress] = p.Amount
		total += p.Amount
	}

	// Alice has 1200 difficulty out of 2000 total = 60%.
	// Bob has 800 difficulty out of 2000 total = 40%.
	aliceExpected := uint64(600_000_000_000) // 60% of 1 XMR
	bobExpected := uint64(400_000_000_000)   // 40% of 1 XMR

	// Due to map iteration order the last miner gets the dust remainder.
	// Total must equal the full reward regardless.
	if total != 1_000_000_000_000 {
		t.Errorf("total payments = %d, want 1_000_000_000_000", total)
	}

	// Check approximate values (within 1 piconero of rounding).
	if a := paymentMap["4addr_alice"]; a < aliceExpected-1 || a > aliceExpected+1 {
		t.Errorf("alice payment = %d, want ~%d", a, aliceExpected)
	}
	if b := paymentMap["4addr_bob"]; b < bobExpected-1 || b > bobExpected+1 {
		t.Errorf("bob payment = %d, want ~%d", b, bobExpected)
	}
}

func TestExtractPaymentsNoShares(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedTestBlock(t, pool, ctx, 3100000, 1_000_000_000_000)
	// No shares seeded.

	payments, err := scanner.ExtractPayments(ctx, pool, nil, 3100000, "testhash")
	if err != nil {
		t.Fatalf("ExtractPayments: %v", err)
	}

	if payments != nil && len(payments) != 0 {
		t.Errorf("expected nil/empty payments with no shares, got %d", len(payments))
	}
}

func TestRecordPaymentsIntegration(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	seedTestBlock(t, pool, ctx, 3100000, 1_000_000_000_000)
	seedTestShares(t, pool, ctx)

	payments, err := scanner.ExtractPayments(ctx, pool, nil, 3100000, "testhash1234567890testhash1234567890testhash1234567890testhash12")
	if err != nil {
		t.Fatalf("ExtractPayments: %v", err)
	}

	// recordPayments is unexported, but we can verify by checking the payments table
	// after the full scanner pipeline runs. For now, verify the extraction works.
	if len(payments) == 0 {
		t.Fatal("expected payments from extraction")
	}

	// Verify each payment has the correct block reference.
	for _, p := range payments {
		if p.MainHeight != 3100000 {
			t.Errorf("payment MainHeight = %d, want 3100000", p.MainHeight)
		}
		if p.Amount == 0 {
			t.Errorf("payment for %s has zero amount", p.MinerAddress)
		}
	}
}
