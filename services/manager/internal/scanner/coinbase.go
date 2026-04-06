package scanner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/monerod"
)

// Payment represents a detected coinbase payment to a miner.
type Payment struct {
	MinerAddress string
	Amount       uint64 // atomic units (1 XMR = 1e12)
	MainHeight   uint64
	MainHash     string
	XMRUSDPrice  *float64 // XMR/USD spot price at time of payment (nil if unavailable)
	XMRCADPrice  *float64 // XMR/CAD spot price at time of payment (nil if unavailable)
}

// ExtractPayments extracts payments from a coinbase transaction's outputs.
//
// Since we cannot derive stealth addresses without miner view keys, we use a
// proportional share calculation: each miner's payment is estimated based on
// their difficulty contribution to the PPLNS window relative to the total
// coinbase reward recorded in p2pool_blocks.
//
// This approach cross-references:
//  1. The block height with p2pool_blocks to get the total coinbase_reward.
//  2. The miner difficulty contributions from p2pool_shares.
//  3. Distributes the reward proportionally.
func ExtractPayments(ctx context.Context, pool *pgxpool.Pool, txJSON *monerod.TxJSON, mainHeight uint64, mainHash string) ([]Payment, error) {
	// Get the recorded coinbase reward for this block.
	var coinbaseReward uint64
	err := pool.QueryRow(ctx,
		`SELECT coinbase_reward FROM p2pool_blocks WHERE main_height = $1`,
		mainHeight,
	).Scan(&coinbaseReward)
	if err != nil {
		return nil, fmt.Errorf("querying coinbase reward for height %d: %w", mainHeight, err)
	}

	// Fetch the difficulty contribution per miner from shares in the PPLNS window.
	// We use the most recent shares that were in the window when this block was found.
	minerDifficulties, totalDifficulty, err := fetchMinerDifficulties(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("fetching miner difficulties: %w", err)
	}

	if totalDifficulty == 0 {
		slog.Warn("no shares found in PPLNS window, cannot calculate payments",
			slog.Uint64("height", mainHeight),
		)
		return nil, nil
	}

	// Calculate proportional payments.
	payments := make([]Payment, 0, len(minerDifficulties))
	var distributedTotal uint64

	i := 0
	for address, difficulty := range minerDifficulties {
		i++
		var amount uint64

		if i == len(minerDifficulties) {
			// Last miner gets the remainder to avoid rounding dust.
			amount = coinbaseReward - distributedTotal
		} else {
			// Proportional share: (miner_difficulty / total_difficulty) * reward.
			amount = (difficulty * coinbaseReward) / totalDifficulty
		}

		distributedTotal += amount

		if amount == 0 {
			continue
		}

		payments = append(payments, Payment{
			MinerAddress: address,
			Amount:       amount,
			MainHeight:   mainHeight,
			MainHash:     mainHash,
		})
	}

	return payments, nil
}

// fetchMinerDifficulties queries the most recent miner_hashrate bucket to get
// the relative hashrate contribution per miner, and the most recent pool-wide
// hashrate from pool_stats_snapshots as the total. This ensures each miner's
// share is calculated against the entire P2Pool sidechain, not just the local
// node's stratum connections.
func fetchMinerDifficulties(ctx context.Context, pool *pgxpool.Pool) (map[string]uint64, uint64, error) {
	rows, err := pool.Query(ctx,
		`SELECT miner_address, hashrate
		 FROM miner_hashrate
		 WHERE bucket_time = (SELECT MAX(bucket_time) FROM miner_hashrate)`,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("querying miner hashrates: %w", err)
	}
	defer rows.Close()

	miners := make(map[string]uint64)

	for rows.Next() {
		var address string
		var hashrate uint64
		if err := rows.Scan(&address, &hashrate); err != nil {
			return nil, 0, fmt.Errorf("scanning miner hashrate row: %w", err)
		}
		miners[address] = hashrate
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating miner hashrate rows: %w", err)
	}

	// Use the pool-wide hashrate as the denominator so payments reflect
	// each miner's share of the entire sidechain, not just the local node.
	var poolHashrate uint64
	err = pool.QueryRow(ctx,
		`SELECT pool_hashrate FROM pool_stats_snapshots
		 ORDER BY created_at DESC LIMIT 1`,
	).Scan(&poolHashrate)
	if err != nil {
		return nil, 0, fmt.Errorf("querying pool hashrate from snapshots: %w", err)
	}

	return miners, poolHashrate, nil
}

// recordPayments inserts payments into the payments table.
func recordPayments(ctx context.Context, pool *pgxpool.Pool, payments []Payment) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning payment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, p := range payments {
		_, err := tx.Exec(ctx,
			`INSERT INTO payments (miner_address, amount, main_height, main_hash, xmr_usd_price, xmr_cad_price)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			p.MinerAddress,
			p.Amount,
			p.MainHeight,
			p.MainHash,
			p.XMRUSDPrice,
			p.XMRCADPrice,
		)
		if err != nil {
			return fmt.Errorf("inserting payment for %s at height %d: %w", p.MinerAddress, p.MainHeight, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing payment transaction: %w", err)
	}

	return nil
}
