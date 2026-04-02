package subscription

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/cache"
)

const (
	// cacheTTL is how long subscription status is cached in Redis.
	cacheTTL = 30 * time.Second

	// apiKeyLength is the number of random bytes for API key generation.
	apiKeyLength = 32
)

// Service provides subscription CRUD and tier-checking operations.
type Service struct {
	pool   *pgxpool.Pool
	cache  *cache.Store
	logger *slog.Logger
}

// NewService creates a new subscription service.
func NewService(pool *pgxpool.Pool, cacheStore *cache.Store, logger *slog.Logger) *Service {
	return &Service{
		pool:   pool,
		cache:  cacheStore,
		logger: logger.With(slog.String("component", "subscription")),
	}
}

// GetOrCreateSubscription returns the subscription for a miner address,
// creating a free-tier entry if none exists.
func (s *Service) GetOrCreateSubscription(ctx context.Context, minerAddress string) (*Subscription, error) {
	var sub Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, miner_address, tier, api_key_hash, email, expires_at, grace_until, created_at, updated_at
		 FROM subscriptions WHERE miner_address = $1`,
		minerAddress,
	).Scan(&sub.ID, &sub.MinerAddress, &sub.Tier, &sub.APIKeyHash, &sub.Email,
		&sub.ExpiresAt, &sub.GraceUntil, &sub.CreatedAt, &sub.UpdatedAt)

	if err == nil {
		return &sub, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("querying subscription for %s: %w", minerAddress, err)
	}

	// Create a free-tier entry.
	err = s.pool.QueryRow(ctx,
		`INSERT INTO subscriptions (miner_address, tier)
		 VALUES ($1, 'free')
		 ON CONFLICT (miner_address) DO UPDATE SET miner_address = $1
		 RETURNING id, miner_address, tier, api_key_hash, email, expires_at, grace_until, created_at, updated_at`,
		minerAddress,
	).Scan(&sub.ID, &sub.MinerAddress, &sub.Tier, &sub.APIKeyHash, &sub.Email,
		&sub.ExpiresAt, &sub.GraceUntil, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating free subscription for %s: %w", minerAddress, err)
	}

	return &sub, nil
}

// GetSubscriptionByAddress returns the subscription for a miner, or nil if not found.
func (s *Service) GetSubscriptionByAddress(ctx context.Context, minerAddress string) (*Subscription, error) {
	var sub Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, miner_address, tier, api_key_hash, email, expires_at, grace_until, created_at, updated_at
		 FROM subscriptions WHERE miner_address = $1`,
		minerAddress,
	).Scan(&sub.ID, &sub.MinerAddress, &sub.Tier, &sub.APIKeyHash, &sub.Email,
		&sub.ExpiresAt, &sub.GraceUntil, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying subscription by address %s: %w", minerAddress, err)
	}
	return &sub, nil
}

// GetSubscriptionByAPIKey looks up a subscription by API key.
// The provided key is hashed and compared against stored hashes.
func (s *Service) GetSubscriptionByAPIKey(ctx context.Context, apiKey string) (*Subscription, error) {
	hash := HashAPIKey(apiKey)

	var sub Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, miner_address, tier, api_key_hash, email, expires_at, grace_until, created_at, updated_at
		 FROM subscriptions WHERE api_key_hash = $1`,
		hash,
	).Scan(&sub.ID, &sub.MinerAddress, &sub.Tier, &sub.APIKeyHash, &sub.Email,
		&sub.ExpiresAt, &sub.GraceUntil, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying subscription by API key: %w", err)
	}
	return &sub, nil
}

// GenerateAPIKey creates a new API key for a paid subscription.
// Returns the plaintext key (shown once to the user) and stores the hash.
//
// Proof of ownership is required:
//   - First-time: provide a confirmed subscription tx_hash for this address.
//   - Regeneration: provide the existing API key via existingKey.
func (s *Service) GenerateAPIKey(ctx context.Context, minerAddress, txHash, existingKey string) (string, error) {
	// Verify the subscription is paid and active.
	sub, err := s.GetSubscriptionByAddress(ctx, minerAddress)
	if err != nil {
		return "", fmt.Errorf("checking subscription for API key: %w", err)
	}
	if sub == nil || !sub.IsActive() {
		return "", fmt.Errorf("active supporter or champion subscription required for API key generation")
	}

	// Proof of ownership check.
	hasExistingKey := sub.APIKeyHash != nil

	if hasExistingKey {
		// Regeneration: require existing API key.
		if existingKey == "" {
			return "", fmt.Errorf("existing API key required to regenerate")
		}
		if HashAPIKey(existingKey) != *sub.APIKeyHash {
			return "", fmt.Errorf("invalid existing API key")
		}
	} else {
		// First-time: require a confirmed subscription tx_hash.
		if txHash == "" {
			return "", fmt.Errorf("confirmed subscription tx_hash required for first-time API key generation")
		}
		var confirmed bool
		err := s.pool.QueryRow(ctx,
			`SELECT confirmed FROM subscription_payments
			 WHERE miner_address = $1 AND tx_hash = $2`,
			minerAddress, txHash,
		).Scan(&confirmed)
		if err != nil || !confirmed {
			return "", fmt.Errorf("tx_hash not found or not confirmed for this address")
		}
	}

	// Generate random key.
	keyBytes := make([]byte, apiKeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("generating random API key: %w", err)
	}
	plaintext := hex.EncodeToString(keyBytes)
	hash := HashAPIKey(plaintext)

	// Store the hash.
	_, err = s.pool.Exec(ctx,
		`UPDATE subscriptions SET api_key_hash = $1, updated_at = NOW() WHERE miner_address = $2`,
		hash, minerAddress,
	)
	if err != nil {
		return "", fmt.Errorf("storing API key hash for %s: %w", minerAddress, err)
	}

	// Invalidate cached tier.
	s.invalidateCache(ctx, minerAddress)

	s.logger.Info("API key generated", slog.String("miner_address", minerAddress))
	return plaintext, nil
}

// CheckTier returns the effective tier for a miner address, using Redis cache.
func (s *Service) CheckTier(ctx context.Context, minerAddress string) (Tier, error) {
	// Try cache first.
	cacheKey := "sub:" + minerAddress
	var cached CachedTier
	found, err := s.cache.Get(ctx, cacheKey, &cached)
	if err != nil {
		s.logger.Debug("cache get failed for subscription", "key", cacheKey, "error", err)
	}
	if found {
		if TierIncludes(cached.Tier, TierSupporter) && cached.GraceUntil != nil && time.Now().Before(*cached.GraceUntil) {
			return cached.Tier, nil
		}
		if cached.Tier == TierFree {
			return TierFree, nil
		}
		// Supporter/Champion but expired — fall through to DB check for fresh data.
	}

	// DB lookup.
	sub, err := s.GetSubscriptionByAddress(ctx, minerAddress)
	if err != nil {
		return TierFree, fmt.Errorf("checking tier for %s: %w", minerAddress, err)
	}

	tier := TierFree
	var graceUntil *time.Time
	if sub != nil && sub.IsActive() {
		tier = sub.Tier
		graceUntil = sub.GraceUntil
	}

	// Update cache.
	if cacheErr := s.cache.Set(ctx, cacheKey, CachedTier{Tier: tier, GraceUntil: graceUntil}, cacheTTL); cacheErr != nil {
		s.logger.Debug("cache set failed for subscription", "key", cacheKey, "error", cacheErr)
	}

	return tier, nil
}

// CheckTierByAPIKey resolves tier from an API key, using Redis cache.
func (s *Service) CheckTierByAPIKey(ctx context.Context, apiKey string) (Tier, string, error) {
	hash := HashAPIKey(apiKey)
	cacheKey := "apikey:" + hash[:16] // Use prefix of hash as cache key.

	// Try cache.
	var cached struct {
		Address    string     `json:"address"`
		Tier       Tier       `json:"tier"`
		GraceUntil *time.Time `json:"grace_until,omitempty"`
	}
	found, err := s.cache.Get(ctx, cacheKey, &cached)
	if err == nil && found {
		if TierIncludes(cached.Tier, TierSupporter) && cached.GraceUntil != nil && time.Now().Before(*cached.GraceUntil) {
			return cached.Tier, cached.Address, nil
		}
	}

	// DB lookup.
	sub, err := s.GetSubscriptionByAPIKey(ctx, apiKey)
	if err != nil {
		return TierFree, "", fmt.Errorf("checking tier by API key: %w", err)
	}
	if sub == nil {
		return TierFree, "", nil
	}

	tier := TierFree
	if sub.IsActive() {
		tier = sub.Tier
	}

	// Update cache.
	cacheVal := struct {
		Address    string     `json:"address"`
		Tier       Tier       `json:"tier"`
		GraceUntil *time.Time `json:"grace_until,omitempty"`
	}{
		Address:    sub.MinerAddress,
		Tier:       tier,
		GraceUntil: sub.GraceUntil,
	}
	if cacheErr := s.cache.Set(ctx, cacheKey, cacheVal, cacheTTL); cacheErr != nil {
		s.logger.Debug("cache set failed for API key", "error", cacheErr)
	}

	return tier, sub.MinerAddress, nil
}

// GetStatus returns the subscription status for display to the miner.
func (s *Service) GetStatus(ctx context.Context, minerAddress string) (*SubscriptionStatus, error) {
	sub, err := s.GetOrCreateSubscription(ctx, minerAddress)
	if err != nil {
		return nil, fmt.Errorf("getting subscription status for %s: %w", minerAddress, err)
	}

	return &SubscriptionStatus{
		MinerAddress: sub.MinerAddress,
		Tier:         sub.Tier,
		Active:       sub.IsActive(),
		ExpiresAt:    sub.ExpiresAt,
		GraceUntil:   sub.GraceUntil,
		HasAPIKey:    sub.APIKeyHash != nil,
	}, nil
}

// GetPayments returns subscription payment history for a miner.
func (s *Service) GetPayments(ctx context.Context, minerAddress string, limit, offset int) ([]SubPayment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, miner_address, tx_hash, amount, xmr_usd_price, xmr_cad_price, confirmed, main_height, created_at
		 FROM subscription_payments
		 WHERE miner_address = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		minerAddress, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("querying subscription payments for %s: %w", minerAddress, err)
	}
	defer rows.Close()

	var payments []SubPayment
	for rows.Next() {
		var p SubPayment
		if err := rows.Scan(&p.ID, &p.MinerAddress, &p.TxHash, &p.Amount,
			&p.XMRUSDPrice, &p.XMRCADPrice, &p.Confirmed, &p.MainHeight, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning subscription payment row: %w", err)
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating subscription payment rows: %w", err)
	}

	return payments, nil
}

// GetAllConfirmedPayments returns all confirmed subscription payments across all miners.
// If year is non-zero, filters to that calendar year (UTC). Used for admin income reporting.
func (s *Service) GetAllConfirmedPayments(ctx context.Context, year int) ([]SubPayment, error) {
	var query string
	var args []interface{}

	if year > 0 {
		query = `SELECT id, miner_address, tx_hash, amount, xmr_usd_price, xmr_cad_price, confirmed, main_height, created_at
		 FROM subscription_payments
		 WHERE confirmed = TRUE
		   AND created_at >= make_date($1, 1, 1)
		   AND created_at < make_date($1 + 1, 1, 1)
		 ORDER BY created_at ASC`
		args = []interface{}{year}
	} else {
		query = `SELECT id, miner_address, tx_hash, amount, xmr_usd_price, xmr_cad_price, confirmed, main_height, created_at
		 FROM subscription_payments
		 WHERE confirmed = TRUE
		 ORDER BY created_at ASC`
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying all confirmed subscription payments: %w", err)
	}
	defer rows.Close()

	var payments []SubPayment
	for rows.Next() {
		var p SubPayment
		if err := rows.Scan(&p.ID, &p.MinerAddress, &p.TxHash, &p.Amount,
			&p.XMRUSDPrice, &p.XMRCADPrice, &p.Confirmed, &p.MainHeight, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning subscription payment row: %w", err)
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating subscription payment rows: %w", err)
	}

	return payments, nil
}

// invalidateCache removes cached subscription data for a miner.
func (s *Service) invalidateCache(ctx context.Context, minerAddress string) {
	if err := s.cache.Delete(ctx, "sub:"+minerAddress); err != nil {
		s.logger.Debug("cache delete failed", "key", "sub:"+minerAddress, "error", err)
	}
}

// HashAPIKey returns the SHA-256 hex digest of an API key.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
