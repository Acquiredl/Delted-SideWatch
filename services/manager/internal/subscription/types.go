package subscription

import "time"

// Tier represents a subscription tier level.
type Tier string

const (
	TierFree Tier = "free"
	TierPaid Tier = "paid"
)

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
	if s.Tier != TierPaid {
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

// DefaultMinUSD is the minimum USD equivalent to accept as a valid subscription payment.
const DefaultMinUSD = 4.0 // $5 target with 20% tolerance
