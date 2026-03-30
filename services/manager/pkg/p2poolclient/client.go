package p2poolclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Client is an HTTP client for the P2Pool local API.
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

// GetPoolStats fetches GET /api/pool/stats.
func (c *Client) GetPoolStats(ctx context.Context) (*PoolStats, error) {
	stats, err := doGet[PoolStats](ctx, c, "/api/pool/stats")
	if err != nil {
		return nil, fmt.Errorf("fetching pool stats: %w", err)
	}
	return &stats, nil
}

// GetShares fetches GET /api/shares (current PPLNS window).
func (c *Client) GetShares(ctx context.Context) ([]Share, error) {
	shares, err := doGet[[]Share](ctx, c, "/api/shares")
	if err != nil {
		return nil, fmt.Errorf("fetching shares: %w", err)
	}
	return shares, nil
}

// GetFoundBlocks fetches GET /api/found_blocks.
func (c *Client) GetFoundBlocks(ctx context.Context) ([]FoundBlock, error) {
	blocks, err := doGet[[]FoundBlock](ctx, c, "/api/found_blocks")
	if err != nil {
		return nil, fmt.Errorf("fetching found blocks: %w", err)
	}
	return blocks, nil
}

// GetWorkerStats fetches GET /api/worker_stats.
func (c *Client) GetWorkerStats(ctx context.Context) (WorkerStats, error) {
	stats, err := doGet[WorkerStats](ctx, c, "/api/worker_stats")
	if err != nil {
		return nil, fmt.Errorf("fetching worker stats: %w", err)
	}
	return stats, nil
}

// GetPeers fetches GET /api/p2p/peers.
func (c *Client) GetPeers(ctx context.Context) ([]Peer, error) {
	peers, err := doGet[[]Peer](ctx, c, "/api/p2p/peers")
	if err != nil {
		return nil, fmt.Errorf("fetching peers: %w", err)
	}
	return peers, nil
}
