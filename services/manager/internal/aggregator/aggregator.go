package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/cache"
)

// PoolOverview is the aggregated pool stats returned by GetPoolStats.
type PoolOverview struct {
	TotalMiners          int        `json:"total_miners"`
	TotalHashrate        uint64     `json:"total_hashrate"`
	BlocksFound          int        `json:"blocks_found"`
	LastBlockFoundAt     *time.Time `json:"last_block_found_at"`
	TotalPaid            uint64     `json:"total_paid"`
	Sidechain            string     `json:"sidechain"`
	SidechainHeight      uint64     `json:"sidechain_height"`
	SidechainDifficulty  uint64     `json:"sidechain_difficulty"`
}

// MinerOverview is the aggregated stats for a single miner.
type MinerOverview struct {
	Address         string     `json:"address"`
	CurrentHashrate uint64     `json:"current_hashrate"`
	AverageHashrate uint64     `json:"average_hashrate"`
	TotalShares     int        `json:"total_shares"`
	TotalPaid       uint64     `json:"total_paid"`
	LastShareAt     *time.Time `json:"last_share_at"`
	LastPaymentAt   *time.Time `json:"last_payment_at"`
}

// MinerPayment represents a single payment row for the API.
type MinerPayment struct {
	Amount      uint64    `json:"amount"`
	MainHeight  uint64    `json:"main_height"`
	XMRUSDPrice *float64  `json:"xmr_usd_price,omitempty"`
	XMRCADPrice *float64  `json:"xmr_cad_price,omitempty"`
	PaidAt      time.Time `json:"paid_at"`
}

// HashratePoint is one data point in a hashrate timeseries.
type HashratePoint struct {
	Hashrate   uint64    `json:"hashrate"`
	BucketTime time.Time `json:"bucket_time"`
}

// FoundBlock represents a block for the API blocks list.
type FoundBlock struct {
	MainHeight      uint64    `json:"main_height"`
	MainHash        string    `json:"main_hash"`
	SidechainHeight uint64    `json:"sidechain_height"`
	CoinbaseReward  uint64    `json:"coinbase_reward"`
	Effort          *float64  `json:"effort,omitempty"`
	FoundAt         time.Time `json:"found_at"`
}

// SidechainShare represents a recent share for the sidechain page.
type SidechainShare struct {
	MinerAddress    string    `json:"miner_address"`
	WorkerName      *string   `json:"worker_name,omitempty"`
	Sidechain       string    `json:"sidechain"`
	SidechainHeight uint64    `json:"sidechain_height"`
	Difficulty      uint64    `json:"difficulty"`
	CreatedAt       time.Time `json:"created_at"`
}

// PoolStatsPoint is one data point in the pool stats timeseries.
type PoolStatsPoint struct {
	PoolHashrate        uint64    `json:"pool_hashrate"`
	PoolMiners          int       `json:"pool_miners"`
	SidechainHeight     uint64    `json:"sidechain_height"`
	SidechainDifficulty uint64    `json:"sidechain_difficulty"`
	CreatedAt           time.Time `json:"created_at"`
}

// LocalWorker represents a miner connected to the local stratum with recent hashrate data.
type LocalWorker struct {
	MinerAddress    string    `json:"miner_address"`
	CurrentHashrate uint64    `json:"current_hashrate"`
	LastSeen        time.Time `json:"last_seen"`
}

const (
	poolStatsCacheKey = "pool:stats"
	poolStatsCacheTTL = 15 * time.Second
)

// Aggregator provides query methods for the API layer.
type Aggregator struct {
	pool      *pgxpool.Pool
	cache     *cache.Store // may be nil (e.g. in tests without Redis)
	sidechain string
	logger    *slog.Logger
}

// New creates a new Aggregator.
func New(pool *pgxpool.Pool, cacheStore *cache.Store, sidechain string, logger *slog.Logger) *Aggregator {
	return &Aggregator{
		pool:      pool,
		cache:     cacheStore,
		sidechain: sidechain,
		logger:    logger,
	}
}

// GetPoolStats returns aggregated pool statistics.
// Pool hashrate and miner count come from the latest pool_stats_snapshot
// (populated by the indexer from P2Pool's data-api). Blocks and payments
// come from the existing tables (populated by the coinbase scanner).
func (a *Aggregator) GetPoolStats(ctx context.Context) (*PoolOverview, error) {
	overview := &PoolOverview{Sidechain: a.sidechain}

	// Pool hashrate, miner count, sidechain height and difficulty from latest snapshot.
	err := a.pool.QueryRow(ctx,
		`SELECT COALESCE(pool_hashrate, 0), COALESCE(pool_miners, 0),
		        COALESCE(sidechain_height, 0), COALESCE(sidechain_difficulty, 0)
		 FROM pool_stats_snapshots
		 WHERE sidechain = $1
		 ORDER BY created_at DESC LIMIT 1`,
		a.sidechain).Scan(&overview.TotalHashrate, &overview.TotalMiners,
		&overview.SidechainHeight, &overview.SidechainDifficulty)
	if err != nil {
		// No snapshots yet — fall back to zero values.
		a.logger.Debug("no pool stats snapshots yet", "err", err)
	}

	// Blocks found (all time).
	err = a.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM p2pool_blocks`).Scan(&overview.BlocksFound)
	if err != nil {
		return nil, fmt.Errorf("querying blocks found: %w", err)
	}

	// Last block found timestamp.
	err = a.pool.QueryRow(ctx,
		`SELECT MAX(found_at) FROM p2pool_blocks`).Scan(&overview.LastBlockFoundAt)
	if err != nil {
		return nil, fmt.Errorf("querying last block: %w", err)
	}

	// Total paid out (all time, atomic units).
	err = a.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payments`).Scan(&overview.TotalPaid)
	if err != nil {
		return nil, fmt.Errorf("querying total paid: %w", err)
	}

	return overview, nil
}

// GetPoolStatsCached returns pool stats from Redis if available, falling back
// to Postgres on cache miss. Both the WS hub and HTTP handler use this to
// avoid redundant database queries.
func (a *Aggregator) GetPoolStatsCached(ctx context.Context) (*PoolOverview, error) {
	if a.cache != nil {
		var cached PoolOverview
		found, err := a.cache.Get(ctx, poolStatsCacheKey, &cached)
		if err != nil {
			a.logger.Warn("cache get failed for pool stats", "error", err)
		} else if found {
			return &cached, nil
		}
	}

	stats, err := a.GetPoolStats(ctx)
	if err != nil {
		return nil, err
	}

	if a.cache != nil {
		if err := a.cache.Set(ctx, poolStatsCacheKey, stats, poolStatsCacheTTL); err != nil {
			a.logger.Warn("cache set failed for pool stats", "error", err)
		}
	}

	return stats, nil
}

// minerAddressCondition returns a SQL WHERE fragment and argument for matching
// miner addresses. P2Pool truncates wallet addresses in its data-api, so
// miner_hashrate stores prefixes. When a user enters a full address we match
// with "full_address LIKE stored_prefix || '%'".
func minerAddressCondition(column, paramNum string) string {
	return fmt.Sprintf("($%s = %s OR $%s LIKE %s || '%%')", paramNum, column, paramNum, column)
}

// GetMinerStats returns stats for a specific miner address.
// Supports both exact matches and prefix matching (P2Pool truncates addresses).
func (a *Aggregator) GetMinerStats(ctx context.Context, address string) (*MinerOverview, error) {
	mo := &MinerOverview{Address: address}

	// Current hashrate: latest bucket for this miner.
	// Match where stored address equals input OR input starts with stored prefix.
	err := a.pool.QueryRow(ctx,
		`SELECT COALESCE(hashrate, 0) FROM miner_hashrate
		 WHERE (`+minerAddressCondition("miner_address", "1")+`) AND sidechain = $2
		 ORDER BY bucket_time DESC LIMIT 1`,
		address, a.sidechain).Scan(&mo.CurrentHashrate)
	if err != nil {
		// No rows is not fatal — miner may not have hashrate data yet.
		a.logger.Debug("no current hashrate for miner", "address", address, "err", err)
		mo.CurrentHashrate = 0
	}

	// Average hashrate over the last 24 hours.
	err = a.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(hashrate), 0)::BIGINT FROM miner_hashrate
		 WHERE (`+minerAddressCondition("miner_address", "1")+`) AND sidechain = $2
		   AND bucket_time > NOW() - INTERVAL '24 hours'`,
		address, a.sidechain).Scan(&mo.AverageHashrate)
	if err != nil {
		a.logger.Debug("no average hashrate for miner", "address", address, "err", err)
		mo.AverageHashrate = 0
	}

	// Total shares — p2pool_shares may be empty (P2Pool API doesn't expose individual shares).
	err = a.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM p2pool_shares
		 WHERE (`+minerAddressCondition("miner_address", "1")+`) AND sidechain = $2`,
		address, a.sidechain).Scan(&mo.TotalShares)
	if err != nil {
		return nil, fmt.Errorf("querying total shares for miner %s: %w", address, err)
	}

	// Total paid (atomic units).
	err = a.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM payments
		 WHERE `+minerAddressCondition("miner_address", "1"),
		address).Scan(&mo.TotalPaid)
	if err != nil {
		return nil, fmt.Errorf("querying total paid for miner %s: %w", address, err)
	}

	// Last share timestamp.
	err = a.pool.QueryRow(ctx,
		`SELECT MAX(created_at) FROM p2pool_shares
		 WHERE (`+minerAddressCondition("miner_address", "1")+`) AND sidechain = $2`,
		address, a.sidechain).Scan(&mo.LastShareAt)
	if err != nil {
		return nil, fmt.Errorf("querying last share for miner %s: %w", address, err)
	}

	// Last payment timestamp.
	err = a.pool.QueryRow(ctx,
		`SELECT MAX(created_at) FROM payments
		 WHERE `+minerAddressCondition("miner_address", "1"),
		address).Scan(&mo.LastPaymentAt)
	if err != nil {
		return nil, fmt.Errorf("querying last payment for miner %s: %w", address, err)
	}

	return mo, nil
}

// GetMinerPayments returns paginated payment history for a miner.
// If maxAge is non-zero, only payments within the last maxAge duration are returned.
func (a *Aggregator) GetMinerPayments(ctx context.Context, address string, limit, offset int, maxAge time.Duration) ([]MinerPayment, error) {
	var query string
	var args []interface{}

	if maxAge > 0 {
		query = `SELECT amount, main_height, xmr_usd_price, xmr_cad_price, created_at
		 FROM payments
		 WHERE miner_address = $1
		   AND created_at > NOW() - make_interval(secs => $4)
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`
		args = []interface{}{address, limit, offset, int(maxAge.Seconds())}
	} else {
		query = `SELECT amount, main_height, xmr_usd_price, xmr_cad_price, created_at
		 FROM payments
		 WHERE miner_address = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`
		args = []interface{}{address, limit, offset}
	}

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying payments for miner %s: %w", address, err)
	}
	defer rows.Close()

	var payments []MinerPayment
	for rows.Next() {
		var p MinerPayment
		if err := rows.Scan(&p.Amount, &p.MainHeight, &p.XMRUSDPrice, &p.XMRCADPrice, &p.PaidAt); err != nil {
			return nil, fmt.Errorf("scanning payment row: %w", err)
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating payment rows: %w", err)
	}

	return payments, nil
}

// GetMinerHashrate returns hashrate timeseries for a miner over the given number of hours.
func (a *Aggregator) GetMinerHashrate(ctx context.Context, address string, hours int) ([]HashratePoint, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT hashrate, bucket_time
		 FROM miner_hashrate
		 WHERE (`+minerAddressCondition("miner_address", "1")+`) AND sidechain = $2
		   AND bucket_time > NOW() - make_interval(hours => $3)
		 ORDER BY bucket_time ASC`,
		address, a.sidechain, hours)
	if err != nil {
		return nil, fmt.Errorf("querying hashrate for miner %s: %w", address, err)
	}
	defer rows.Close()

	var points []HashratePoint
	for rows.Next() {
		var p HashratePoint
		if err := rows.Scan(&p.Hashrate, &p.BucketTime); err != nil {
			return nil, fmt.Errorf("scanning hashrate row: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating hashrate rows: %w", err)
	}

	return points, nil
}

// GetBlocks returns paginated found blocks.
func (a *Aggregator) GetBlocks(ctx context.Context, limit, offset int) ([]FoundBlock, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT main_height, main_hash, sidechain_height, coinbase_reward, effort, found_at
		 FROM p2pool_blocks
		 ORDER BY found_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying found blocks: %w", err)
	}
	defer rows.Close()

	var blocks []FoundBlock
	for rows.Next() {
		var b FoundBlock
		if err := rows.Scan(&b.MainHeight, &b.MainHash, &b.SidechainHeight, &b.CoinbaseReward, &b.Effort, &b.FoundAt); err != nil {
			return nil, fmt.Errorf("scanning block row: %w", err)
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating block rows: %w", err)
	}

	return blocks, nil
}

// GetSidechainShares returns recent sidechain shares, paginated.
func (a *Aggregator) GetSidechainShares(ctx context.Context, limit, offset int) ([]SidechainShare, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT miner_address, worker_name, sidechain, sidechain_height, difficulty, created_at
		 FROM p2pool_shares
		 WHERE sidechain = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		a.sidechain, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying sidechain shares: %w", err)
	}
	defer rows.Close()

	var shares []SidechainShare
	for rows.Next() {
		var s SidechainShare
		if err := rows.Scan(&s.MinerAddress, &s.WorkerName, &s.Sidechain, &s.SidechainHeight, &s.Difficulty, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning share row: %w", err)
		}
		shares = append(shares, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating share rows: %w", err)
	}

	return shares, nil
}

// GetLocalWorkers returns miners with recent hashrate data (seen in the last hour).
func (a *Aggregator) GetLocalWorkers(ctx context.Context) ([]LocalWorker, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT miner_address, hashrate, bucket_time
		 FROM miner_hashrate
		 WHERE sidechain = $1
		   AND bucket_time > NOW() - INTERVAL '1 hour'
		   AND (miner_address, bucket_time) IN (
		     SELECT miner_address, MAX(bucket_time)
		     FROM miner_hashrate
		     WHERE sidechain = $1
		       AND bucket_time > NOW() - INTERVAL '1 hour'
		     GROUP BY miner_address
		   )
		 ORDER BY hashrate DESC`,
		a.sidechain)
	if err != nil {
		return nil, fmt.Errorf("querying local workers: %w", err)
	}
	defer rows.Close()

	var workers []LocalWorker
	for rows.Next() {
		var w LocalWorker
		if err := rows.Scan(&w.MinerAddress, &w.CurrentHashrate, &w.LastSeen); err != nil {
			return nil, fmt.Errorf("scanning worker row: %w", err)
		}
		workers = append(workers, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating worker rows: %w", err)
	}

	return workers, nil
}

// GetPoolStatsHistory returns pool stats timeseries for the given number of hours.
// Points are sampled at ~5 minute intervals by selecting every 10th snapshot
// (snapshots are recorded every 30s, so 10 * 30s = 5min).
func (a *Aggregator) GetPoolStatsHistory(ctx context.Context, hours int) ([]PoolStatsPoint, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT pool_hashrate, pool_miners, sidechain_height, sidechain_difficulty, created_at
		 FROM (
		   SELECT *, ROW_NUMBER() OVER (ORDER BY created_at ASC) AS rn
		   FROM pool_stats_snapshots
		   WHERE sidechain = $1
		     AND created_at > NOW() - make_interval(hours => $2)
		 ) sub
		 WHERE rn % 10 = 0
		 ORDER BY created_at ASC`,
		a.sidechain, hours)
	if err != nil {
		return nil, fmt.Errorf("querying pool stats history: %w", err)
	}
	defer rows.Close()

	var points []PoolStatsPoint
	for rows.Next() {
		var p PoolStatsPoint
		if err := rows.Scan(&p.PoolHashrate, &p.PoolMiners, &p.SidechainHeight, &p.SidechainDifficulty, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning pool stats point: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pool stats points: %w", err)
	}

	return points, nil
}

// GetMinerPaymentsForExport returns ALL payments for a miner (for CSV tax export).
// No pagination — caller should stream or buffer as needed.
func (a *Aggregator) GetMinerPaymentsForExport(ctx context.Context, address string) ([]MinerPayment, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT amount, main_height, xmr_usd_price, xmr_cad_price, created_at
		 FROM payments
		 WHERE miner_address = $1
		 ORDER BY created_at ASC`,
		address)
	if err != nil {
		return nil, fmt.Errorf("querying payments for export (miner %s): %w", address, err)
	}
	defer rows.Close()

	var payments []MinerPayment
	for rows.Next() {
		var p MinerPayment
		if err := rows.Scan(&p.Amount, &p.MainHeight, &p.XMRUSDPrice, &p.XMRCADPrice, &p.PaidAt); err != nil {
			return nil, fmt.Errorf("scanning export payment row: %w", err)
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating export payment rows: %w", err)
	}

	return payments, nil
}
