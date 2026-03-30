package walletrpc

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

// RPCError represents a JSON-RPC error returned by monero-wallet-rpc.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *RPCError) Error() string {
	return fmt.Sprintf("wallet-rpc error %d: %s", e.Code, e.Message)
}

// Address holds a primary address and its index.
type Address struct {
	Address      string `json:"address"`
	AddressIndex uint32 `json:"address_index"`
	Label        string `json:"label"`
	Used         bool   `json:"used"`
}

// CreateAddressResult is the response from create_address.
type CreateAddressResult struct {
	Address      string `json:"address"`
	AddressIndex uint32 `json:"address_index"`
}

// GetAddressResult is the response from get_address.
type GetAddressResult struct {
	Address   string    `json:"address"`
	Addresses []Address `json:"addresses"`
}

// Transfer represents a single incoming or outgoing transfer.
type Transfer struct {
	Address       string       `json:"address"`
	Amount        uint64       `json:"amount"`
	Confirmations uint64       `json:"confirmations"`
	Height        uint64       `json:"height"`
	TxHash        string       `json:"txid"`
	SubaddrIndex  SubaddrIndex `json:"subaddr_index"`
	Timestamp     uint64       `json:"timestamp"`
	Type          string       `json:"type"`
	UnlockTime    uint64       `json:"unlock_time"`
}

// SubaddrIndex identifies a subaddress by account and address index.
type SubaddrIndex struct {
	Major uint32 `json:"major"`
	Minor uint32 `json:"minor"`
}

// GetTransfersResult is the response from get_transfers.
type GetTransfersResult struct {
	In      []Transfer `json:"in"`
	Out     []Transfer `json:"out"`
	Pending []Transfer `json:"pending"`
	Pool    []Transfer `json:"pool"`
}

// GetHeightResult is the response from get_height.
type GetHeightResult struct {
	Height uint64 `json:"height"`
}
