package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// coingeckoURL is the CoinGecko simple price endpoint for Monero.
	coingeckoURL = "https://api.coingecko.com/api/v3/simple/price?ids=monero&vs_currencies=usd,cad"

	// defaultCacheTTL is the default minimum time between API calls.
	defaultCacheTTL = 120 * time.Second

	// rateLimitBackoff is the extended TTL when a 429 response is received.
	rateLimitBackoff = 5 * time.Minute

	// httpTimeout is the timeout for CoinGecko HTTP requests.
	httpTimeout = 10 * time.Second
)

// Price holds the current XMR prices in fiat currencies.
type Price struct {
	USD float64 `json:"usd"`
	CAD float64 `json:"cad"`
}

// coingeckoResponse matches the JSON structure returned by CoinGecko.
type coingeckoResponse struct {
	Monero struct {
		USD float64 `json:"usd"`
		CAD float64 `json:"cad"`
	} `json:"monero"`
}

// PriceOracle fetches XMR/USD and XMR/CAD prices from CoinGecko.
// It caches the result and is safe for concurrent use.
type PriceOracle struct {
	httpClient *http.Client
	logger     *slog.Logger
	url        string

	mu        sync.RWMutex
	lastPrice *Price
	lastFetch time.Time
	cacheTTL  time.Duration
}

// NewPriceOracle creates a new PriceOracle with default settings.
// If url is empty, the default CoinGecko API URL is used.
func NewPriceOracle(logger *slog.Logger, url string) *PriceOracle {
	if url == "" {
		url = coingeckoURL
	}
	return &PriceOracle{
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
		url:        url,
		cacheTTL:   defaultCacheTTL,
	}
}

// NewPriceOracleWithTTL creates a new PriceOracle with a custom cache TTL.
// The minimum enforced TTL is 60 seconds. If url is empty, the default
// CoinGecko API URL is used.
func NewPriceOracleWithTTL(logger *slog.Logger, url string, ttl time.Duration) *PriceOracle {
	if ttl < 60*time.Second {
		ttl = 60 * time.Second
	}
	if url == "" {
		url = coingeckoURL
	}
	return &PriceOracle{
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
		url:        url,
		cacheTTL:   ttl,
	}
}

// GetPrice returns the current XMR price. If the cached price is still fresh,
// it returns the cached value. Otherwise it fetches a new price from CoinGecko.
// On network or rate-limit errors, the cached value is returned if available.
func (po *PriceOracle) GetPrice(ctx context.Context) (*Price, error) {
	// Fast path: return cached value if still fresh.
	po.mu.RLock()
	if po.lastPrice != nil && time.Since(po.lastFetch) < po.cacheTTL {
		price := *po.lastPrice
		po.mu.RUnlock()
		return &price, nil
	}
	po.mu.RUnlock()

	// Slow path: fetch new price.
	price, err := po.fetch(ctx)
	if err != nil {
		// If we have a cached value, return it with a logged warning.
		po.mu.RLock()
		cached := po.lastPrice
		po.mu.RUnlock()

		if cached != nil {
			po.logger.Warn("failed to fetch price, returning cached value",
				"err", err,
				"cached_age", time.Since(po.lastFetch))
			return &Price{USD: cached.USD, CAD: cached.CAD}, nil
		}
		return nil, fmt.Errorf("fetching XMR price: %w", err)
	}

	return price, nil
}

// fetch performs the HTTP request to CoinGecko and updates the cache.
func (po *PriceOracle) fetch(ctx context.Context) (*Price, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, po.url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := po.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting: extend cache TTL and return cached value if available.
	if resp.StatusCode == http.StatusTooManyRequests {
		po.mu.Lock()
		po.cacheTTL = rateLimitBackoff
		po.mu.Unlock()

		po.logger.Warn("CoinGecko rate limit hit, extending cache TTL",
			"new_ttl", rateLimitBackoff)

		po.mu.RLock()
		cached := po.lastPrice
		po.mu.RUnlock()

		if cached != nil {
			return &Price{USD: cached.USD, CAD: cached.CAD}, nil
		}
		return nil, fmt.Errorf("CoinGecko rate limited (HTTP 429) and no cached price available")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected CoinGecko status: %d", resp.StatusCode)
	}

	var cgResp coingeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&cgResp); err != nil {
		return nil, fmt.Errorf("decoding CoinGecko response: %w", err)
	}

	price := &Price{
		USD: cgResp.Monero.USD,
		CAD: cgResp.Monero.CAD,
	}

	// Update cache.
	po.mu.Lock()
	po.lastPrice = price
	po.lastFetch = time.Now()
	// Reset TTL to default if it was extended by a previous rate limit.
	if po.cacheTTL > defaultCacheTTL {
		po.cacheTTL = defaultCacheTTL
	}
	po.mu.Unlock()

	po.logger.Debug("fetched XMR price", "usd", price.USD, "cad", price.CAD)
	return price, nil
}
