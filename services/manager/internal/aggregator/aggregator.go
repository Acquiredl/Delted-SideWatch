package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/cache"
)

// PoolOverview is the aggregated pool stats returned by GetPoolStats.
type PoolOverview struct {
	TotalMiners         int        `json:"total_miners"`
	TotalHashrate       uint64     `json:"total_hashrate"`
	BlocksFound         int        `json:"blocks_found"`
	LastBlockFoundAt    *time.Time `json:"last_block_found_at"`
	TotalPaid           uint64     `json:"total_paid"`
	Sidechain           string     `json:"sidechain"`
	SidechainHeight     uint64     `json:"sidechain_height"`
	SidechainDifficulty uint64     `json:"sidechain_difficulty"`
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
	UncleRate24h    *float64   `json:"uncle_rate_24h,omitempty"`
}

// UncleRatePoint is one data point in an uncle rate timeseries.
type UncleRatePoint struct {
	UncleRate  float64   `json:"uncle_rate"`
	Total      int       `json:"total_shares"`
	Uncles     int       `json:"uncle_shares"`
	BucketTime time.Time `json:"bucket_time"`
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
	MainHeight         uint64    `json:"main_height"`
	MainHash           string    `json:"main_hash"`
	SidechainHeight    uint64    `json:"sidechain_height"`
	CoinbaseReward     uint64    `json:"coinbase_reward"`
	Effort             *float64  `json:"effort,omitempty"`
	CoinbasePrivateKey *string   `json:"coinbase_private_key,omitempty"`
	FoundAt            time.Time `json:"found_at"`
}

// SidechainShare represents a recent share for the sidechain page.
type SidechainShare struct {
	MinerAddress    string    `json:"miner_address"`
	WorkerName      *string   `json:"worker_name,omitempty"`
	Sidechain       string    `json:"sidechain"`
	SidechainHeight uint64    `json:"sidechain_height"`
	Difficulty      uint64    `json:"difficulty"`
	IsUncle         bool      `json:"is_uncle"`
	SoftwareID      *int16    `json:"software_id,omitempty"`
	SoftwareVersion *string   `json:"software_version,omitempty"`
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
// All queries are pipelined in a single pgx.Batch round-trip.
// Supports prefix matching for P2Pool's truncated wallet addresses.
func (a *Aggregator) GetMinerStats(ctx context.Context, address string) (*MinerOverview, error) {
	mo := &MinerOverview{Address: address}

	addrCond := minerAddressCondition("miner_address", "1")

	batch := &pgx.Batch{}

	// 0: Current hashrate
	batch.Queue(`SELECT COALESCE(hashrate, 0) FROM miner_hashrate
		 WHERE (`+addrCond+`) AND sidechain = $2
		 ORDER BY bucket_time DESC LIMIT 1`, address, a.sidechain)

	// 1: Average hashrate (24h)
	batch.Queue(`SELECT COALESCE(AVG(hashrate), 0)::BIGINT FROM miner_hashrate
		 WHERE (`+addrCond+`) AND sidechain = $2
		   AND bucket_time > NOW() - INTERVAL '24 hours'`, address, a.sidechain)

	// 2: Total shares
	batch.Queue(`SELECT COUNT(*) FROM p2pool_shares
		 WHERE (`+addrCond+`) AND sidechain = $2`, address, a.sidechain)

	// 3: Total paid
	batch.Queue(`SELECT COALESCE(SUM(amount), 0) FROM payments
		 WHERE `+addrCond, address)

	// 4: Last share timestamp
	batch.Queue(`SELECT MAX(created_at) FROM p2pool_shares
		 WHERE (`+addrCond+`) AND sidechain = $2`, address, a.sidechain)

	// 5: Last payment timestamp
	batch.Queue(`SELECT MAX(created_at) FROM payments
		 WHERE `+addrCond, address)

	// 6: Uncle rate (24h)
	batch.Queue(`SELECT COUNT(*), COUNT(*) FILTER (WHERE is_uncle = true)
		 FROM p2pool_shares
		 WHERE (`+addrCond+`) AND sidechain = $2
		   AND created_at > NOW() - INTERVAL '24 hours'`, address, a.sidechain)

	br := a.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	// 0: Current hashrate (no-rows is non-fatal).
	if err := br.QueryRow().Scan(&mo.CurrentHashrate); err != nil {
		a.logger.Debug("no current hashrate for miner", "address", address, "err", err)
		mo.CurrentHashrate = 0
	}

	// 1: Average hashrate (no-rows is non-fatal).
	if err := br.QueryRow().Scan(&mo.AverageHashrate); err != nil {
		a.logger.Debug("no average hashrate for miner", "address", address, "err", err)
		mo.AverageHashrate = 0
	}

	// 2: Total shares.
	if err := br.QueryRow().Scan(&mo.TotalShares); err != nil {
		return nil, fmt.Errorf("querying total shares for miner %s: %w", address, err)
	}

	// 3: Total paid.
	if err := br.QueryRow().Scan(&mo.TotalPaid); err != nil {
		return nil, fmt.Errorf("querying total paid for miner %s: %w", address, err)
	}

	// 4: Last share timestamp.
	if err := br.QueryRow().Scan(&mo.LastShareAt); err != nil {
		return nil, fmt.Errorf("querying last share for miner %s: %w", address, err)
	}

	// 5: Last payment timestamp.
	if err := br.QueryRow().Scan(&mo.LastPaymentAt); err != nil {
		return nil, fmt.Errorf("querying last payment for miner %s: %w", address, err)
	}

	// 6: Uncle rate (no-rows is non-fatal).
	var totalShares24h, uncleShares24h int
	if err := br.QueryRow().Scan(&totalShares24h, &uncleShares24h); err != nil {
		a.logger.Debug("no uncle data for miner", "address", address, "err", err)
	} else if totalShares24h > 0 {
		rate := float64(uncleShares24h) / float64(totalShares24h)
		mo.UncleRate24h = &rate
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
		`SELECT main_height, main_hash, sidechain_height, coinbase_reward, effort, coinbase_private_key, found_at
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
		if err := rows.Scan(&b.MainHeight, &b.MainHash, &b.SidechainHeight, &b.CoinbaseReward, &b.Effort, &b.CoinbasePrivateKey, &b.FoundAt); err != nil {
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
		`SELECT miner_address, worker_name, sidechain, sidechain_height, difficulty,
		        is_uncle, software_id, software_version, created_at
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
		if err := rows.Scan(&s.MinerAddress, &s.WorkerName, &s.Sidechain, &s.SidechainHeight, &s.Difficulty,
			&s.IsUncle, &s.SoftwareID, &s.SoftwareVersion, &s.CreatedAt); err != nil {
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

// WeeklyMiner represents a miner active in the last 7 days.
type WeeklyMiner struct {
	Address     string    `json:"address"`
	ShareCount  int       `json:"share_count"`
	LastShareAt time.Time `json:"last_share_at"`
}

// GetWeeklyMiners returns miners who submitted at least one share in the last 7 days.
func (a *Aggregator) GetWeeklyMiners(ctx context.Context) ([]WeeklyMiner, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT miner_address, COUNT(*) AS share_count, MAX(created_at) AS last_share
		 FROM p2pool_shares
		 WHERE sidechain = $1 AND created_at > NOW() - INTERVAL '7 days'
		 GROUP BY miner_address
		 ORDER BY share_count DESC`,
		a.sidechain)
	if err != nil {
		return nil, fmt.Errorf("querying weekly miners: %w", err)
	}
	defer rows.Close()

	var miners []WeeklyMiner
	for rows.Next() {
		var m WeeklyMiner
		if err := rows.Scan(&m.Address, &m.ShareCount, &m.LastShareAt); err != nil {
			return nil, fmt.Errorf("scanning weekly miner row: %w", err)
		}
		miners = append(miners, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating weekly miner rows: %w", err)
	}

	return miners, nil
}

// GetMinerUncleRate returns uncle rate timeseries in 1-hour buckets for a miner.
func (a *Aggregator) GetMinerUncleRate(ctx context.Context, address string, hours int) ([]UncleRatePoint, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT
		   date_trunc('hour', created_at) AS bucket,
		   COUNT(*) AS total,
		   COUNT(*) FILTER (WHERE is_uncle = true) AS uncles
		 FROM p2pool_shares
		 WHERE miner_address = $1 AND sidechain = $2
		   AND created_at > NOW() - make_interval(hours => $3)
		 GROUP BY bucket
		 ORDER BY bucket ASC`,
		address, a.sidechain, hours)
	if err != nil {
		return nil, fmt.Errorf("querying uncle rate for miner %s: %w", address, err)
	}
	defer rows.Close()

	var points []UncleRatePoint
	for rows.Next() {
		var p UncleRatePoint
		if err := rows.Scan(&p.BucketTime, &p.Total, &p.Uncles); err != nil {
			return nil, fmt.Errorf("scanning uncle rate row: %w", err)
		}
		if p.Total > 0 {
			p.UncleRate = float64(p.Uncles) / float64(p.Total)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating uncle rate rows: %w", err)
	}

	return points, nil
}

// MinerWorker represents a single worker's stats for a miner.
type MinerWorker struct {
	WorkerName  string    `json:"worker_name"`
	Shares      int       `json:"shares"`
	LastShareAt time.Time `json:"last_share_at"`
}

// GetMinerWorkers returns per-worker share breakdown for a miner address.
func (a *Aggregator) GetMinerWorkers(ctx context.Context, address string) ([]MinerWorker, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT COALESCE(worker_name, 'default') AS wname, COUNT(*) AS shares, MAX(created_at) AS last_share
		 FROM p2pool_shares
		 WHERE miner_address = $1 AND sidechain = $2
		 GROUP BY wname
		 ORDER BY shares DESC`,
		address, a.sidechain)
	if err != nil {
		return nil, fmt.Errorf("querying workers for miner %s: %w", address, err)
	}
	defer rows.Close()

	var workers []MinerWorker
	for rows.Next() {
		var w MinerWorker
		if err := rows.Scan(&w.WorkerName, &w.Shares, &w.LastShareAt); err != nil {
			return nil, fmt.Errorf("scanning worker row: %w", err)
		}
		workers = append(workers, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating worker rows: %w", err)
	}

	return workers, nil
}

// GetMinerPaymentsForExport returns payments for a miner (for CSV tax export).
// If year is non-zero, only payments from that calendar year (UTC) are returned.
// No pagination — caller should stream or buffer as needed.
func (a *Aggregator) GetMinerPaymentsForExport(ctx context.Context, address string, year int) ([]MinerPayment, error) {
	var query string
	var args []interface{}

	if year > 0 {
		query = `SELECT amount, main_height, xmr_usd_price, xmr_cad_price, created_at
		 FROM payments
		 WHERE miner_address = $1
		   AND created_at >= make_date($2, 1, 1)
		   AND created_at < make_date($2 + 1, 1, 1)
		 ORDER BY created_at ASC`
		args = []interface{}{address, year}
	} else {
		query = `SELECT amount, main_height, xmr_usd_price, xmr_cad_price, created_at
		 FROM payments
		 WHERE miner_address = $1
		 ORDER BY created_at ASC`
		args = []interface{}{address}
	}

	rows, err := a.pool.Query(ctx, query, args...)
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

// PaymentYearSummary represents aggregated payment totals for a single year.
type PaymentYearSummary struct {
	Year         int      `json:"year"`
	PaymentCount int      `json:"payment_count"`
	TotalAtomic  uint64   `json:"total_atomic"`
	TotalCAD     *float64 `json:"total_cad,omitempty"`
	TotalUSD     *float64 `json:"total_usd,omitempty"`
}

// GetMinerPaymentSummary returns per-year payment totals for a miner.
func (a *Aggregator) GetMinerPaymentSummary(ctx context.Context, address string) ([]PaymentYearSummary, error) {
	rows, err := a.pool.Query(ctx,
		`SELECT EXTRACT(YEAR FROM created_at)::INT AS yr,
		        COUNT(*) AS cnt,
		        SUM(amount) AS total_atomic,
		        SUM(CASE WHEN xmr_cad_price IS NOT NULL
		            THEN (amount / 1e12) * xmr_cad_price END) AS total_cad,
		        SUM(CASE WHEN xmr_usd_price IS NOT NULL
		            THEN (amount / 1e12) * xmr_usd_price END) AS total_usd
		 FROM payments
		 WHERE miner_address = $1
		 GROUP BY yr
		 ORDER BY yr ASC`,
		address)
	if err != nil {
		return nil, fmt.Errorf("querying payment summary for miner %s: %w", address, err)
	}
	defer rows.Close()

	var summaries []PaymentYearSummary
	for rows.Next() {
		var s PaymentYearSummary
		if err := rows.Scan(&s.Year, &s.PaymentCount, &s.TotalAtomic, &s.TotalCAD, &s.TotalUSD); err != nil {
			return nil, fmt.Errorf("scanning payment summary row: %w", err)
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating payment summary rows: %w", err)
	}

	return summaries, nil
}
