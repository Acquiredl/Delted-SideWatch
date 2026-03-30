package walletrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Client is a JSON-RPC client for monero-wallet-rpc (view-only mode).
type Client struct {
	rpcURL     string
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a new monero-wallet-rpc client.
func New(rpcURL string, logger *slog.Logger) *Client {
	return &Client{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With(slog.String("component", "walletrpc")),
	}
}

// callRPC performs a JSON-RPC call and decodes the result into T.
func callRPC[T any](ctx context.Context, c *Client, method string, params interface{}) (T, error) {
	var zero T

	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      "0",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return zero, fmt.Errorf("marshaling wallet-rpc request for %s: %w", method, err)
	}

	url := c.rpcURL + "/json_rpc"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return zero, fmt.Errorf("creating wallet-rpc request for %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("calling wallet-rpc", slog.String("method", method))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("calling wallet-rpc %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("calling wallet-rpc %s: unexpected status %d", method, resp.StatusCode)
	}

	var rpcResp rpcResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return zero, fmt.Errorf("decoding wallet-rpc response for %s: %w", method, err)
	}

	if rpcResp.Error != nil {
		return zero, fmt.Errorf("calling wallet-rpc %s: %w", method, rpcResp.Error)
	}

	return rpcResp.Result, nil
}

// CreateAddress generates a new subaddress under account 0.
// The label is stored in the wallet for reference.
func (c *Client) CreateAddress(ctx context.Context, label string) (*CreateAddressResult, error) {
	params := map[string]interface{}{
		"account_index": 0,
		"label":         label,
	}

	result, err := callRPC[CreateAddressResult](ctx, c, "create_address", params)
	if err != nil {
		return nil, fmt.Errorf("creating subaddress: %w", err)
	}
	return &result, nil
}

// GetAddress returns the primary address and all subaddresses for account 0.
func (c *Client) GetAddress(ctx context.Context) (*GetAddressResult, error) {
	params := map[string]interface{}{
		"account_index": 0,
	}

	result, err := callRPC[GetAddressResult](ctx, c, "get_address", params)
	if err != nil {
		return nil, fmt.Errorf("getting address: %w", err)
	}
	return &result, nil
}

// GetTransfers returns incoming and outgoing transfers.
// Set minHeight to filter transfers from a specific block height.
func (c *Client) GetTransfers(ctx context.Context, minHeight uint64) (*GetTransfersResult, error) {
	params := map[string]interface{}{
		"in":               true,
		"out":              false,
		"pending":          true,
		"pool":             true,
		"filter_by_height": minHeight > 0,
		"min_height":       minHeight,
	}

	result, err := callRPC[GetTransfersResult](ctx, c, "get_transfers", params)
	if err != nil {
		return nil, fmt.Errorf("getting transfers from height %d: %w", minHeight, err)
	}
	return &result, nil
}

// GetHeight returns the wallet's current blockchain height.
func (c *Client) GetHeight(ctx context.Context) (uint64, error) {
	result, err := callRPC[GetHeightResult](ctx, c, "get_height", nil)
	if err != nil {
		return 0, fmt.Errorf("getting wallet height: %w", err)
	}
	return result.Height, nil
}
