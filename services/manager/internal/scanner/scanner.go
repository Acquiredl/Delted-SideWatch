package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/monerod"
)

// Scanner watches for new Monero blocks and extracts coinbase payments.
// It implements a confirmation depth buffer to avoid recording payments
// from orphaned blocks.
type Scanner struct {
	monerod      *monerod.Client
	pool         *pgxpool.Pool
	oracle       *PriceOracle
	confirmDepth uint64 // number of confirmations required (default 10)
	logger       *slog.Logger

	mu            sync.Mutex
	pendingBlocks map[uint64]bool // heights awaiting confirmation
}

// NewScanner creates a new coinbase scanner. The oracle parameter is optional;
// if nil, payments will be recorded without fiat price data.
func NewScanner(monerodClient *monerod.Client, pool *pgxpool.Pool, oracle *PriceOracle, confirmDepth uint64, logger *slog.Logger) *Scanner {
	if confirmDepth == 0 {
		confirmDepth = 10
	}
	return &Scanner{
		monerod:       monerodClient,
		pool:          pool,
		oracle:        oracle,
		confirmDepth:  confirmDepth,
		logger:        logger.With(slog.String("component", "scanner")),
		pendingBlocks: make(map[uint64]bool),
	}
}

// HandleNewBlock is called when a new block is detected (via ZMQ or polling).
// It adds the block to the pending set, then checks all pending blocks to see
// if any have reached the required confirmation depth.
func (s *Scanner) HandleNewBlock(ctx context.Context, height uint64) error {
	s.mu.Lock()
	s.pendingBlocks[height] = true
	s.mu.Unlock()

	s.logger.Info("new block added to pending set",
		slog.Uint64("height", height),
		slog.Uint64("confirm_depth", s.confirmDepth),
	)

	if err := s.checkConfirmations(ctx, height); err != nil {
		return fmt.Errorf("checking confirmations at height %d: %w", height, err)
	}

	return nil
}

// checkConfirmations promotes pending blocks that have enough confirmations.
// For each pending block where (currentHeight - pendingHeight) >= confirmDepth,
// it processes the block and removes it from the pending set.
func (s *Scanner) checkConfirmations(ctx context.Context, currentHeight uint64) error {
	s.mu.Lock()
	var confirmed []uint64
	for h := range s.pendingBlocks {
		if currentHeight-h >= s.confirmDepth {
			confirmed = append(confirmed, h)
		}
	}
	for _, h := range confirmed {
		delete(s.pendingBlocks, h)
	}
	s.mu.Unlock()

	for _, h := range confirmed {
		s.logger.Info("block confirmed, processing coinbase",
			slog.Uint64("height", h),
			slog.Uint64("confirmations", currentHeight-h),
		)
		if err := s.processBlock(ctx, h); err != nil {
			s.logger.Error("failed to process confirmed block",
				slog.Uint64("height", h),
				slog.String("error", err.Error()),
			)
			// Re-add to pending so we can retry on next block.
			s.mu.Lock()
			s.pendingBlocks[h] = true
			s.mu.Unlock()
		}
	}

	return nil
}

// processBlock fetches the block and extracts coinbase payment information.
func (s *Scanner) processBlock(ctx context.Context, height uint64) error {
	// First check if this block is a P2Pool found block by querying our DB.
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM p2pool_blocks WHERE main_height = $1)`,
		height,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking p2pool_blocks for height %d: %w", height, err)
	}

	if !exists {
		// Not a P2Pool block, nothing to process.
		s.logger.Debug("block is not a P2Pool block, skipping",
			slog.Uint64("height", height),
		)
		return nil
	}

	// Fetch the full block from monerod to get the coinbase tx hash.
	block, err := s.monerod.GetBlock(ctx, height)
	if err != nil {
		return fmt.Errorf("fetching block at height %d: %w", height, err)
	}

	if block.BlockHeader.OrphanStatus {
		s.logger.Warn("block is orphaned, skipping payment extraction",
			slog.Uint64("height", height),
		)
		return nil
	}

	// Fetch the coinbase transaction.
	txResult, err := s.monerod.GetTransactions(ctx, []string{block.MinerTxHash})
	if err != nil {
		return fmt.Errorf("fetching coinbase tx %s: %w", block.MinerTxHash, err)
	}

	if len(txResult.Txs) == 0 {
		return fmt.Errorf("coinbase tx %s not found in monerod response", block.MinerTxHash)
	}

	// Parse the coinbase tx JSON.
	var txJSON monerod.TxJSON
	if err := json.Unmarshal([]byte(txResult.Txs[0].AsJSON), &txJSON); err != nil {
		return fmt.Errorf("parsing coinbase tx JSON for %s: %w", block.MinerTxHash, err)
	}

	// Extract payments by matching against known miners.
	payments, err := ExtractPayments(ctx, s.pool, &txJSON, height, block.BlockHeader.Hash)
	if err != nil {
		return fmt.Errorf("extracting payments from block %d: %w", height, err)
	}

	if len(payments) == 0 {
		s.logger.Info("no payments extracted from block",
			slog.Uint64("height", height),
		)
		return nil
	}

	// Fetch current XMR price and attach to all payments.
	if s.oracle != nil {
		price, err := s.oracle.GetPrice(ctx)
		if err != nil {
			s.logger.Warn("failed to fetch XMR price, recording payments without fiat values",
				slog.Uint64("height", height),
				slog.String("error", err.Error()),
			)
		} else {
			for i := range payments {
				payments[i].XMRUSDPrice = &price.USD
				payments[i].XMRCADPrice = &price.CAD
			}
		}
	}

	// Record the payments in the database.
	if err := recordPayments(ctx, s.pool, payments); err != nil {
		return fmt.Errorf("recording payments for block %d: %w", height, err)
	}

	s.logger.Info("payments recorded",
		slog.Uint64("height", height),
		slog.Int("payment_count", len(payments)),
	)

	return nil
}
