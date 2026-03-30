package monerod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Client is a JSON-RPC client for monerod.
type Client struct {
	rpcURL     string
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a new monerod RPC client.
func New(rpcURL string, logger *slog.Logger) *Client {
	return &Client{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
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
		return zero, fmt.Errorf("marshaling RPC request for %s: %w", method, err)
	}

	url := c.rpcURL + "/json_rpc"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return zero, fmt.Errorf("creating RPC request for %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("calling monerod RPC", slog.String("method", method))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("calling RPC %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("calling RPC %s: unexpected status %d", method, resp.StatusCode)
	}

	var rpcResp rpcResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return zero, fmt.Errorf("decoding RPC response for %s: %w", method, err)
	}

	if rpcResp.Error != nil {
		return zero, fmt.Errorf("calling RPC %s: %w", method, rpcResp.Error)
	}

	return rpcResp.Result, nil
}

// GetLastBlockHeader fetches the most recent block header.
func (c *Client) GetLastBlockHeader(ctx context.Context) (*BlockHeader, error) {
	result, err := callRPC[BlockHeaderResult](ctx, c, "get_last_block_header", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching last block header: %w", err)
	}
	return &result.BlockHeader, nil
}

// GetBlockHeaderByHeight fetches a block header at a specific height.
func (c *Client) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*BlockHeader, error) {
	params := map[string]uint64{"height": height}

	result, err := callRPC[BlockHeaderResult](ctx, c, "get_block_header_by_height", params)
	if err != nil {
		return nil, fmt.Errorf("fetching block header at height %d: %w", height, err)
	}
	return &result.BlockHeader, nil
}

// GetBlock fetches a full block at a specific height (including miner_tx_hash).
func (c *Client) GetBlock(ctx context.Context, height uint64) (*Block, error) {
	params := map[string]uint64{"height": height}

	result, err := callRPC[Block](ctx, c, "get_block", params)
	if err != nil {
		return nil, fmt.Errorf("fetching block at height %d: %w", height, err)
	}
	return &result, nil
}

// GetTransactions fetches transactions by hash.
// This uses the non-JSON-RPC endpoint at /get_transactions.
func (c *Client) GetTransactions(ctx context.Context, txHashes []string) (*TransactionResult, error) {
	reqBody := getTransactionsRequest{
		TxHashes:     txHashes,
		DecodeAsJSON: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling get_transactions request: %w", err)
	}

	url := c.rpcURL + "/get_transactions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating get_transactions request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("fetching transactions", slog.Int("count", len(txHashes)))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching transactions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching transactions: unexpected status %d", resp.StatusCode)
	}

	var result TransactionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding get_transactions response: %w", err)
	}

	if result.Status != "" && result.Status != "OK" {
		return nil, fmt.Errorf("fetching transactions: monerod returned status %s", result.Status)
	}

	return &result, nil
}
