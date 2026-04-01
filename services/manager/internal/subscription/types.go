package subscription

import "time"

// Tier represents a subscription tier level.
type Tier string

const (
	TierFree      Tier = "free"
	TierSupporter Tier = "supporter"
	TierChampion  Tier = "champion"
)

// tierHierarchy maps each tier to its rank for comparison.
var tierHierarchy = map[Tier]int{
	TierFree:      0,
	TierSupporter: 1,
	TierChampion:  2,
}

// TierIncludes returns true if actual tier is at or above required tier.
// Example: TierIncludes(TierChampion, TierSupporter) == true.
func TierIncludes(actual Tier, required Tier) bool {
	return tierHierarchy[actual] >= tierHierarchy[required]
}

// TierForAmount returns the tier earned by a USD payment amount.
// Champion requires $5+ (with tolerance), Supporter requires $1+ (with tolerance).
// Below minimum returns TierFree (payment too small to activate).
func TierForAmount(usdValue float64) Tier {
	if usdValue >= ChampionMinUSD {
		return TierChampion
	}
	if usdValue >= SupporterMinUSD {
		return TierSupporter
	}
	return TierFree
}

// Subscription represents a miner's subscription state.
type Subscription struct {
	ID           int64      `json:"id"`
	MinerAddress string     `json:"miner_address"`
	Tier         Tier       `json:"tier"`
	APIKeyHash   *string    `json:"-"` // never exposed in API responses
	Email        *string    `json:"email,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	GraceUntil   *time.Time `json:"grace_until,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// SubAddress maps a miner to a unique wallet subaddress.
type SubAddress struct {
	ID              int64     `json:"id"`
	MinerAddress    string    `json:"miner_address"`
	Subaddress      string    `json:"subaddress"`
	SubaddressIndex int       `json:"subaddress_index"`
	CreatedAt       time.Time `json:"created_at"`
}

// SubPayment is a subscription payment detected on-chain.
type SubPayment struct {
	ID           int64     `json:"id"`
	MinerAddress string    `json:"miner_address"`
	TxHash       string    `json:"tx_hash"`
	Amount       uint64    `json:"amount"`
	XMRUSDPrice  *float64  `json:"xmr_usd_price,omitempty"`
	Confirmed    bool      `json:"confirmed"`
	MainHeight   *uint64   `json:"main_height,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SubscriptionStatus is the API response for a subscription status check.
type SubscriptionStatus struct {
	MinerAddress string     `json:"miner_address"`
	Tier         Tier       `json:"tier"`
	Active       bool       `json:"active"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	GraceUntil   *time.Time `json:"grace_until,omitempty"`
	HasAPIKey    bool       `json:"has_api_key"`
}

// PaymentAddress is the API response for a subaddress assignment.
type PaymentAddress struct {
	MinerAddress string `json:"miner_address"`
	Subaddress   string `json:"subaddress"`
	AmountXMR    string `json:"suggested_amount_xmr"`
	AmountUSD    string `json:"amount_usd"`
}

// CachedTier is the minimal subscription info stored in Redis.
type CachedTier struct {
	Tier       Tier       `json:"tier"`
	GraceUntil *time.Time `json:"grace_until,omitempty"`
}

// IsActive returns true if the subscription grants paid-tier access right now.
func (s *Subscription) IsActive() bool {
	if !TierIncludes(s.Tier, TierSupporter) {
		return false
	}
	if s.GraceUntil == nil {
		return false
	}
	return time.Now().Before(*s.GraceUntil)
}

// DefaultSubscriptionDays is the default subscription duration.
const DefaultSubscriptionDays = 30

// DefaultGraceHours is the grace period after expiry.
const DefaultGraceHours = 48

// SupporterMinUSD is the minimum USD to activate Supporter tier ($1 target with 20% tolerance).
const SupporterMinUSD = 0.80

// ChampionMinUSD is the minimum USD to activate Champion tier ($5 target with 20% tolerance).
const ChampionMinUSD = 4.00

// DefaultMinUSD is the minimum USD to accept as any valid subscription payment.
const DefaultMinUSD = SupporterMinUSD
