package p2poolclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Client is an HTTP client for the P2Pool data-api (served via nginx sidecar).
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a new P2Pool API client.
func New(baseURL string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// doGet performs a GET request and decodes the JSON response into T.
func doGet[T any](ctx context.Context, c *Client, path string) (T, error) {
	var zero T

	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return zero, fmt.Errorf("creating request for %s: %w", path, err)
	}

	c.logger.Debug("requesting p2pool API", slog.String("path", path))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("requesting %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("requesting %s: unexpected status %d", path, resp.StatusCode)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("decoding response from %s: %w", path, err)
	}

	return result, nil
}

// GetPoolStats fetches /pool/stats.
func (c *Client) GetPoolStats(ctx context.Context) (*PoolStats, error) {
	stats, err := doGet[PoolStats](ctx, c, "/pool/stats")
	if err != nil {
		return nil, fmt.Errorf("fetching pool stats: %w", err)
	}
	return &stats, nil
}

// GetNetworkStats fetches /network/stats.
func (c *Client) GetNetworkStats(ctx context.Context) (*NetworkStats, error) {
	stats, err := doGet[NetworkStats](ctx, c, "/network/stats")
	if err != nil {
		return nil, fmt.Errorf("fetching network stats: %w", err)
	}
	return &stats, nil
}

// GetLocalStratum fetches /local/stratum (workers connected to our node).
func (c *Client) GetLocalStratum(ctx context.Context) (*LocalStratum, error) {
	stratum, err := doGet[LocalStratum](ctx, c, "/local/stratum")
	if err != nil {
		return nil, fmt.Errorf("fetching local stratum: %w", err)
	}
	return &stratum, nil
}

// GetLocalP2P fetches /local/p2p (peer connections).
func (c *Client) GetLocalP2P(ctx context.Context) (*LocalP2P, error) {
	p2p, err := doGet[LocalP2P](ctx, c, "/local/p2p")
	if err != nil {
		return nil, fmt.Errorf("fetching local p2p: %w", err)
	}
	return &p2p, nil
}

// GetPeers returns the peer list from /local/p2p as CSV strings.
func (c *Client) GetPeers(ctx context.Context) ([]string, error) {
	p2p, err := c.GetLocalP2P(ctx)
	if err != nil {
		return nil, err
	}
	return p2p.Peers, nil
}
