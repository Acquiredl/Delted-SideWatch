package p2pool

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Indexer polls the P2Pool API and upserts data into Postgres.
type Indexer struct {
	service         *Service
	pool            *pgxpool.Pool
	interval        time.Duration
	lastShareHeight uint64 // tracks highest indexed sidechain height for dedup
	logger          *slog.Logger
}

// NewIndexer creates a new P2Pool indexer.
func NewIndexer(service *Service, pool *pgxpool.Pool, interval time.Duration, logger *slog.Logger) *Indexer {
	return &Indexer{
		service:  service,
		pool:     pool,
		interval: interval,
		logger:   logger.With(slog.String("component", "p2pool-indexer")),
	}
}

// Run starts the indexer polling loop. It blocks until ctx is cancelled.
func (idx *Indexer) Run(ctx context.Context) error {
	idx.logger.Info("starting p2pool indexer",
		slog.String("sidechain", idx.service.Sidechain()),
		slog.Duration("interval", idx.interval),
	)

	// Run once immediately before starting the ticker.
	idx.runCycle(ctx)

	ticker := time.NewTicker(idx.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			idx.logger.Info("p2pool indexer stopping")
			return ctx.Err()
		case <-ticker.C:
			idx.runCycle(ctx)
		}
	}
}

// runCycle performs one indexing cycle: shares + blocks.
func (idx *Indexer) runCycle(ctx context.Context) {
	start := time.Now()

	shareCount, err := idx.indexShares(ctx)
	if err != nil {
		idx.logger.Error("failed to index shares", slog.String("error", err.Error()))
	}

	blockCount, err := idx.indexBlocks(ctx)
	if err != nil {
		idx.logger.Error("failed to index blocks", slog.String("error", err.Error()))
	}

	idx.logger.Info("indexing cycle complete",
		slog.Int("new_shares", shareCount),
		slog.Int("new_blocks", blockCount),
		slog.Duration("elapsed", time.Since(start)),
	)
}

// indexShares fetches current PPLNS window shares and inserts new ones.
// It tracks the last indexed sidechain height in memory to avoid re-inserting
// shares that have already been recorded. On restart, the P2Pool API only
// returns the current PPLNS window, so some overlap is expected and handled
// via the dedup check.
func (idx *Indexer) indexShares(ctx context.Context) (int, error) {
	shares, err := idx.service.FetchShares(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetching shares: %w", err)
	}

	if len(shares) == 0 {
		return 0, nil
	}

	sidechain := idx.service.Sidechain()
	inserted := 0

	for _, share := range shares {
		// Skip shares at or below the last indexed height.
		if share.Height <= idx.lastShareHeight {
			continue
		}

		_, err := idx.pool.Exec(ctx,
			`INSERT INTO p2pool_shares (sidechain, miner_address, worker_name, sidechain_height, difficulty, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			sidechain,
			share.MinerAddress,
			share.WorkerName,
			share.Height,
			share.Difficulty,
			time.Unix(share.Timestamp, 0),
		)
		if err != nil {
			return inserted, fmt.Errorf("inserting share at sidechain height %d: %w", share.Height, err)
		}
		inserted++
	}

	// Update the high-water mark to the maximum sidechain height seen.
	for _, share := range shares {
		if share.Height > idx.lastShareHeight {
			idx.lastShareHeight = share.Height
		}
	}

	return inserted, nil
}

// indexBlocks fetches found blocks and upserts them using ON CONFLICT DO NOTHING
// on main_height (which has a UNIQUE constraint in the schema).
func (idx *Indexer) indexBlocks(ctx context.Context) (int, error) {
	blocks, err := idx.service.FetchFoundBlocks(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetching found blocks: %w", err)
	}

	if len(blocks) == 0 {
		return 0, nil
	}

	inserted := 0

	for _, block := range blocks {
		tag, err := idx.pool.Exec(ctx,
			`INSERT INTO p2pool_blocks (main_height, main_hash, sidechain_height, coinbase_reward, effort, found_at)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (main_height) DO NOTHING`,
			block.MainHeight,
			block.MainHash,
			block.SidechainHeight,
			block.Reward,
			block.Effort,
			time.Unix(block.Timestamp, 0),
		)
		if err != nil {
			return inserted, fmt.Errorf("upserting block at main height %d: %w", block.MainHeight, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
			idx.logger.Info("indexed new block",
				slog.Uint64("main_height", block.MainHeight),
				slog.String("hash", block.MainHash),
				slog.Uint64("reward", block.Reward),
			)
		}
	}

	return inserted, nil
}
