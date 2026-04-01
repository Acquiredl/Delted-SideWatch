package subscription

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/scanner"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/walletrpc"
)

// Scanner watches the subscription wallet for incoming payments and
// activates/extends subscriptions when payments meet the minimum threshold.
type Scanner struct {
	wallet       *walletrpc.Client
	pool         *pgxpool.Pool
	oracle       *scanner.PriceOracle
	confirmDepth uint64
	minUSD       float64
	durationDays int
	graceHours   int
	pollInterval time.Duration
	fundGoalUSD  float64
	infraCostUSD float64
	logger       *slog.Logger
}

// ScannerConfig holds scanner configuration.
type ScannerConfig struct {
	ConfirmDepth uint64
	MinUSD       float64
	DurationDays int
	GraceHours   int
	PollInterval time.Duration
	FundGoalUSD  float64
	InfraCostUSD float64
}

// NewScanner creates a new subscription payment scanner.
func NewScanner(wallet *walletrpc.Client, pool *pgxpool.Pool, oracle *scanner.PriceOracle, cfg ScannerConfig, logger *slog.Logger) *Scanner {
	if cfg.ConfirmDepth == 0 {
		cfg.ConfirmDepth = 10
	}
	if cfg.MinUSD == 0 {
		cfg.MinUSD = DefaultMinUSD
	}
	if cfg.DurationDays == 0 {
		cfg.DurationDays = DefaultSubscriptionDays
	}
	if cfg.GraceHours == 0 {
		cfg.GraceHours = DefaultGraceHours
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}
	if cfg.FundGoalUSD == 0 {
		cfg.FundGoalUSD = 150.0
	}
	if cfg.InfraCostUSD == 0 {
		cfg.InfraCostUSD = 89.0
	}
	return &Scanner{
		wallet:       wallet,
		pool:         pool,
		oracle:       oracle,
		confirmDepth: cfg.ConfirmDepth,
		minUSD:       cfg.MinUSD,
		durationDays: cfg.DurationDays,
		graceHours:   cfg.GraceHours,
		pollInterval: cfg.PollInterval,
		fundGoalUSD:  cfg.FundGoalUSD,
		infraCostUSD: cfg.InfraCostUSD,
		logger:       logger.With(slog.String("component", "sub-scanner")),
	}
}

// Run starts the scanner poll loop. It blocks until ctx is cancelled.
func (s *Scanner) Run(ctx context.Context) error {
	s.logger.Info("subscription scanner starting",
		slog.Duration("poll_interval", s.pollInterval),
		slog.Uint64("confirm_depth", s.confirmDepth),
		slog.Float64("min_usd", s.minUSD),
	)

	// Do an initial scan immediately.
	if err := s.scan(ctx); err != nil {
		s.logger.Error("initial scan failed", slog.String("error", err.Error()))
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("subscription scanner stopped")
			return nil
		case <-ticker.C:
			if err := s.scan(ctx); err != nil {
				s.logger.Error("scan cycle failed", slog.String("error", err.Error()))
			}
		}
	}
}

// scan performs one poll cycle: fetch transfers from wallet-rpc, record new ones,
// promote confirmed ones, and activate subscriptions.
func (s *Scanner) scan(ctx context.Context) error {
	// Get all incoming transfers (confirmed + pool/pending).
	transfers, err := s.wallet.GetTransfers(ctx, 0)
	if err != nil {
		return fmt.Errorf("fetching wallet transfers: %w", err)
	}

	// Process confirmed incoming transfers.
	for _, tx := range transfers.In {
		if err := s.processTransfer(ctx, tx); err != nil {
			s.logger.Error("failed to process transfer",
				slog.String("tx_hash", tx.TxHash),
				slog.String("error", err.Error()),
			)
		}
	}

	// Process pool/pending transfers (record as unconfirmed).
	for _, tx := range transfers.Pool {
		if err := s.recordUnconfirmed(ctx, tx); err != nil {
			s.logger.Error("failed to record pool transfer",
				slog.String("tx_hash", tx.TxHash),
				slog.String("error", err.Error()),
			)
		}
	}
	for _, tx := range transfers.Pending {
		if err := s.recordUnconfirmed(ctx, tx); err != nil {
			s.logger.Error("failed to record pending transfer",
				slog.String("tx_hash", tx.TxHash),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// processTransfer handles a confirmed incoming transfer.
func (s *Scanner) processTransfer(ctx context.Context, tx walletrpc.Transfer) error {
	if tx.Confirmations < s.confirmDepth {
		// Not yet confirmed enough — record as unconfirmed.
		return s.recordUnconfirmed(ctx, tx)
	}

	// Look up the miner address from the subaddress index.
	minerAddress, err := s.lookupMiner(ctx, tx.SubaddrIndex.Minor)
	if err != nil {
		return fmt.Errorf("looking up miner for subaddress index %d: %w", tx.SubaddrIndex.Minor, err)
	}
	if minerAddress == "" {
		// Subaddress index 0 is the primary address — not a subscription payment.
		if tx.SubaddrIndex.Minor == 0 {
			return nil
		}
		s.logger.Warn("no miner found for subaddress index",
			slog.Uint64("subaddr_minor", uint64(tx.SubaddrIndex.Minor)),
			slog.String("tx_hash", tx.TxHash),
		)
		return nil
	}

	// Check if this tx is already recorded as confirmed.
	var confirmed bool
	err = s.pool.QueryRow(ctx,
		`SELECT confirmed FROM subscription_payments WHERE tx_hash = $1`,
		tx.TxHash,
	).Scan(&confirmed)
	if err == nil && confirmed {
		return nil // Already processed.
	}

	// Get current XMR/USD price for amount validation.
	var usdPrice *float64
	if s.oracle != nil {
		price, priceErr := s.oracle.GetPrice(ctx)
		if priceErr != nil {
			s.logger.Warn("failed to fetch price for subscription payment",
				slog.String("tx_hash", tx.TxHash),
				slog.String("error", priceErr.Error()),
			)
		} else {
			usdPrice = &price.USD
		}
	}

	// Upsert the payment as confirmed.
	_, err = s.pool.Exec(ctx,
		`INSERT INTO subscription_payments (miner_address, tx_hash, amount, xmr_usd_price, confirmed, main_height)
		 VALUES ($1, $2, $3, $4, TRUE, $5)
		 ON CONFLICT (tx_hash) DO UPDATE SET confirmed = TRUE, main_height = $5, xmr_usd_price = COALESCE(subscription_payments.xmr_usd_price, $4)`,
		minerAddress, tx.TxHash, tx.Amount, usdPrice, tx.Height,
	)
	if err != nil {
		return fmt.Errorf("upserting confirmed payment %s: %w", tx.TxHash, err)
	}

	s.logger.Info("subscription payment confirmed",
		slog.String("miner_address", minerAddress),
		slog.String("tx_hash", tx.TxHash),
		slog.Uint64("amount", tx.Amount),
		slog.Uint64("height", tx.Height),
	)

	// Determine tier from payment amount.
	usdValue := s.paymentUSDValue(tx.Amount, usdPrice)
	tier := TierForAmount(usdValue)
	if tier == TierFree {
		s.logger.Warn("payment below minimum threshold, not activating subscription",
			slog.String("miner_address", minerAddress),
			slog.String("tx_hash", tx.TxHash),
			slog.Uint64("amount", tx.Amount),
			slog.Float64("usd_value", usdValue),
		)
		return nil
	}

	// Activate or extend the subscription at the determined tier.
	if err := s.activateSubscription(ctx, minerAddress, tier, tx.Amount); err != nil {
		return fmt.Errorf("activating subscription for %s: %w", minerAddress, err)
	}

	// Update the monthly fund totals.
	if err := s.updateFundMonth(ctx, usdValue); err != nil {
		s.logger.Error("failed to update fund month", slog.String("error", err.Error()))
		// Non-fatal: subscription is already activated.
	}

	return nil
}

// recordUnconfirmed records a transfer that hasn't reached confirmation depth yet.
func (s *Scanner) recordUnconfirmed(ctx context.Context, tx walletrpc.Transfer) error {
	minerAddress, err := s.lookupMiner(ctx, tx.SubaddrIndex.Minor)
	if err != nil {
		return fmt.Errorf("looking up miner for subaddress index %d: %w", tx.SubaddrIndex.Minor, err)
	}
	if minerAddress == "" {
		if tx.SubaddrIndex.Minor == 0 {
			return nil
		}
		return nil
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO subscription_payments (miner_address, tx_hash, amount, confirmed, main_height)
		 VALUES ($1, $2, $3, FALSE, $4)
		 ON CONFLICT (tx_hash) DO NOTHING`,
		minerAddress, tx.TxHash, tx.Amount, tx.Height,
	)
	if err != nil {
		return fmt.Errorf("recording unconfirmed payment %s: %w", tx.TxHash, err)
	}

	return nil
}

// lookupMiner returns the miner address associated with a subaddress index.
// Returns empty string if no mapping found.
func (s *Scanner) lookupMiner(ctx context.Context, subaddrIndex uint32) (string, error) {
	var minerAddress string
	err := s.pool.QueryRow(ctx,
		`SELECT miner_address FROM subscription_addresses WHERE subaddress_index = $1`,
		subaddrIndex,
	).Scan(&minerAddress)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", fmt.Errorf("querying subscription_addresses for index %d: %w", subaddrIndex, err)
	}
	return minerAddress, nil
}

// paymentUSDValue returns the USD value of an atomic XMR amount.
// Returns 0 if price is unavailable (caller should treat as supporter-tier).
func (s *Scanner) paymentUSDValue(amountAtomic uint64, usdPrice *float64) float64 {
	if usdPrice == nil || *usdPrice <= 0 {
		// If we can't determine the price, return a value that grants supporter
		// tier. Better to give access than to reject a valid payment.
		return SupporterMinUSD
	}
	amountXMR := float64(amountAtomic) / 1e12
	return amountXMR * (*usdPrice)
}

// activateSubscription sets the miner's subscription to the given tier
// with a new expiry date. If already active, extends from current expiry.
// If the new tier is higher than the existing one, it upgrades.
func (s *Scanner) activateSubscription(ctx context.Context, minerAddress string, tier Tier, contributionAtomic uint64) error {
	now := time.Now()

	// Check if there's an existing active subscription to extend.
	var currentExpiry *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT expires_at FROM subscriptions WHERE miner_address = $1 AND tier IN ('supporter', 'champion')`,
		minerAddress,
	).Scan(&currentExpiry)

	// Determine the base time for extension.
	baseTime := now
	if err == nil && currentExpiry != nil && currentExpiry.After(now) {
		baseTime = *currentExpiry // Extend from current expiry, not from now.
	}

	expiresAt := baseTime.Add(time.Duration(s.durationDays) * 24 * time.Hour)
	graceUntil := expiresAt.Add(time.Duration(s.graceHours) * time.Hour)

	// Upgrade tier if the new payment qualifies for a higher one.
	// Don't downgrade: if existing is champion but new payment is supporter-level, keep champion.
	tierSQL := string(tier)

	_, err = s.pool.Exec(ctx,
		`INSERT INTO subscriptions (miner_address, tier, expires_at, grace_until, extended_retention, retention_since, contribution_amount, updated_at)
		 VALUES ($1, $4, $2, $3, TRUE, NOW(), $5, NOW())
		 ON CONFLICT (miner_address) DO UPDATE
		 SET tier = CASE
		         WHEN EXCLUDED.tier = 'champion' THEN 'champion'
		         WHEN subscriptions.tier = 'champion' THEN 'champion'
		         ELSE EXCLUDED.tier
		     END,
		     expires_at = $2, grace_until = $3,
		     extended_retention = TRUE,
		     retention_since = COALESCE(subscriptions.retention_since, NOW()),
		     contribution_amount = $5,
		     updated_at = NOW()`,
		minerAddress, expiresAt, graceUntil, tierSQL, contributionAtomic,
	)
	if err != nil {
		return fmt.Errorf("upserting subscription for %s: %w", minerAddress, err)
	}

	s.logger.Info("subscription activated",
		slog.String("miner_address", minerAddress),
		slog.String("tier", tierSQL),
		slog.Time("expires_at", expiresAt),
		slog.Time("grace_until", graceUntil),
	)

	return nil
}

// updateFundMonth upserts the current month's fund totals with a new contribution.
func (s *Scanner) updateFundMonth(ctx context.Context, usdValue float64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO node_fund_months (month, goal_usd, infra_cost_usd, total_funded_usd, supporter_count)
		 VALUES (date_trunc('month', NOW())::DATE, $1, $2, $3, 1)
		 ON CONFLICT (month) DO UPDATE
		 SET total_funded_usd = node_fund_months.total_funded_usd + $3,
		     supporter_count = node_fund_months.supporter_count + 1`,
		s.fundGoalUSD, s.infraCostUSD, usdValue,
	)
	if err != nil {
		return fmt.Errorf("upserting fund month: %w", err)
	}
	return nil
}

// AssignSubaddress returns an existing or new subaddress for the given miner.
// This is called when a miner requests their payment address.
func (s *Scanner) AssignSubaddress(ctx context.Context, minerAddress string) (*SubAddress, error) {
	// Check for existing assignment.
	var existing SubAddress
	err := s.pool.QueryRow(ctx,
		`SELECT id, miner_address, subaddress, subaddress_index, created_at
		 FROM subscription_addresses WHERE miner_address = $1`,
		minerAddress,
	).Scan(&existing.ID, &existing.MinerAddress, &existing.Subaddress, &existing.SubaddressIndex, &existing.CreatedAt)
	if err == nil {
		return &existing, nil
	}

	// Generate a new subaddress via wallet-rpc.
	label := fmt.Sprintf("sub-%s", minerAddress[:16])
	result, err := s.wallet.CreateAddress(ctx, label)
	if err != nil {
		return nil, fmt.Errorf("creating subaddress for %s: %w", minerAddress, err)
	}

	// Record the mapping.
	var sa SubAddress
	err = s.pool.QueryRow(ctx,
		`INSERT INTO subscription_addresses (miner_address, subaddress, subaddress_index)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (miner_address) DO UPDATE SET miner_address = $1
		 RETURNING id, miner_address, subaddress, subaddress_index, created_at`,
		minerAddress, result.Address, result.AddressIndex,
	).Scan(&sa.ID, &sa.MinerAddress, &sa.Subaddress, &sa.SubaddressIndex, &sa.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("recording subaddress mapping for %s: %w", minerAddress, err)
	}

	s.logger.Info("subaddress assigned",
		slog.String("miner_address", minerAddress),
		slog.Int("subaddress_index", sa.SubaddressIndex),
	)

	return &sa, nil
}
