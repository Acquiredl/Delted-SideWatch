package subscription

import (
	"context"
	"log/slog"
	"net/http"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	tierKey    contextKey = iota
	addressKey            // miner address resolved from API key
)

// TierFromContext returns the subscription tier stored in the request context.
// Defaults to TierFree if not set.
func TierFromContext(ctx context.Context) Tier {
	tier, ok := ctx.Value(tierKey).(Tier)
	if !ok {
		return TierFree
	}
	return tier
}

// AddressFromContext returns the miner address resolved from an API key, if any.
func AddressFromContext(ctx context.Context) string {
	addr, _ := ctx.Value(addressKey).(string)
	return addr
}

// TierMiddleware resolves the caller's subscription tier and injects it
// into the request context. It checks:
//  1. X-API-Key header → resolve to miner address + tier
//  2. {address} path parameter → look up tier for that address
//
// On any failure, it defaults to free tier (never blocks requests).
func TierMiddleware(svc *Service, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			tier := TierFree
			var resolvedAddress string

			// Check API key first.
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				keyTier, addr, err := svc.CheckTierByAPIKey(ctx, apiKey)
				if err != nil {
					logger.Debug("API key tier check failed", "error", err)
				} else if addr != "" {
					tier = keyTier
					resolvedAddress = addr
				}
			} else {
				// Fall back to path parameter.
				address := r.PathValue("address")
				if address != "" {
					addrTier, err := svc.CheckTier(ctx, address)
					if err != nil {
						logger.Debug("address tier check failed", "address", address, "error", err)
					} else {
						tier = addrTier
					}
				}
			}

			ctx = context.WithValue(ctx, tierKey, tier)
			if resolvedAddress != "" {
				ctx = context.WithValue(ctx, addressKey, resolvedAddress)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTier rejects requests that don't meet the minimum tier requirement.
// Use this to gate specific endpoints (e.g., RequireTier(TierSupporter) for tax-export).
func RequireTier(minTier Tier, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tier := TierFromContext(r.Context())
			if !TierIncludes(tier, minTier) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"` + string(minTier) + ` subscription required"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// FreeTierLimits defines the query limits for free-tier users.
type FreeTierLimits struct {
	MaxHashrateHours int // Maximum hours of hashrate history (e.g., 720 = 30 days)
	MaxPayments      int // Maximum number of payments returned
}

// DefaultFreeLimits returns the standard free-tier limits.
func DefaultFreeLimits() FreeTierLimits {
	return FreeTierLimits{
		MaxHashrateHours: 720, // 30 days
		MaxPayments:      100,
	}
}

// EffectiveHashrateHours returns the max hours based on tier.
func EffectiveHashrateHours(tier Tier, requested int, limits FreeTierLimits) int {
	if TierIncludes(tier, TierSupporter) {
		return requested // No cap for supporter/champion users.
	}
	if requested > limits.MaxHashrateHours {
		return limits.MaxHashrateHours
	}
	return requested
}

// EffectivePaymentLimit returns the max payments based on tier.
func EffectivePaymentLimit(tier Tier, requested int, limits FreeTierLimits) int {
	if TierIncludes(tier, TierSupporter) {
		return requested // No cap for supporter/champion users.
	}
	if requested > limits.MaxPayments {
		return limits.MaxPayments
	}
	return requested
}
