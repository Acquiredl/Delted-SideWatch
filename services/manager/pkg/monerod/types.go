package monerod

import "fmt"

// rpcRequest is the standard JSON-RPC 2.0 request envelope.
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// rpcResponse is a generic JSON-RPC 2.0 response envelope.
type rpcResponse[T any] struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      string    `json:"id"`
	Result  T         `json:"result"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error returned by monerod.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *RPCError) Error() string {
	return fmt.Sprintf("monerod RPC error %d: %s", e.Code, e.Message)
}

// BlockHeader contains metadata about a Monero block.
type BlockHeader struct {
	Height       uint64 `json:"height"`
	Hash         string `json:"hash"`
	Timestamp    uint64 `json:"timestamp"`
	Reward       uint64 `json:"reward"`
	Difficulty   uint64 `json:"difficulty"`
	NumTxes      int    `json:"num_txes"`
	MajorVersion int    `json:"major_version"`
	Nonce        uint64 `json:"nonce"`
	OrphanStatus bool   `json:"orphan_status"`
	Depth        uint64 `json:"depth"`
}

// BlockHeaderResult wraps a single block header in the JSON-RPC result.
type BlockHeaderResult struct {
	BlockHeader BlockHeader `json:"block_header"`
}

// Block represents a full block from get_block, including the coinbase tx hash.
type Block struct {
	Blob        string      `json:"blob"`
	BlockHeader BlockHeader `json:"block_header"`
	JSON        string      `json:"json"` // JSON-encoded block data
	MinerTxHash string      `json:"miner_tx_hash"`
}

// getTransactionsRequest is the POST body for /get_transactions.
type getTransactionsRequest struct {
	TxHashes     []string `json:"txs_hashes"`
	DecodeAsJSON bool     `json:"decode_as_json"`
}

// TransactionResult is the response from /get_transactions.
type TransactionResult struct {
	Txs      []TransactionEntry `json:"txs"`
	MissedTx []string           `json:"missed_tx,omitempty"`
	Status   string             `json:"status"`
}

// TransactionEntry represents a single transaction in the response.
type TransactionEntry struct {
	TxHash string `json:"tx_hash"`
	AsHex  string `json:"as_hex"`
	AsJSON string `json:"as_json"`
	InPool bool   `json:"in_pool"`
}

// TxJSON is the parsed structure from the as_json field of a coinbase tx.
type TxJSON struct {
	Version    int        `json:"version"`
	UnlockTime uint64     `json:"unlock_time"`
	Vin        []TxInput  `json:"vin"`
	Vout       []TxOutput `json:"vout"`
	Extra      []int      `json:"extra"`
}

// TxInput represents a transaction input.
type TxInput struct {
	Gen *TxInputGen `json:"gen,omitempty"`
}

// TxInputGen is a coinbase (generation) input.
type TxInputGen struct {
	Height uint64 `json:"height"`
}

// TxOutput represents a transaction output.
type TxOutput struct {
	Amount uint64         `json:"amount"`
	Target TxOutputTarget `json:"target"`
}

// TxOutputTarget contains the output destination key.
type TxOutputTarget struct {
	TaggedKey *TaggedKey `json:"tagged_key,omitempty"`
	Key       string     `json:"key,omitempty"`
}

// TaggedKey is a tagged output key with view tag (post-HF15 format).
type TaggedKey struct {
	Key     string `json:"key"`
	ViewTag string `json:"view_tag"`
}
