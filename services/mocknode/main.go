// mocknode simulates P2Pool and monerod APIs for local testing.
// P2Pool API on :3333, monerod JSON-RPC on :18081.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// --- Shared state ---

type state struct {
	mu              sync.RWMutex
	mainHeight      uint64
	sidechainHeight uint64
	shares          []share
	foundBlocks     []foundBlock
	miners          []string
}

var s = &state{
	mainHeight:      3_100_000,
	sidechainHeight: 7_500_000,
	miners: []string{
		"4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq",
		"48edfHu7V9Z84YzzMa6fUueoELZ9ZRXq9VetWzYGzKt52XU5xvqgzYnDK9URnRhJP1UdtKNkStMwk9qKBFdvQZCP9tkBavH",
		"44AFFq5kSiGBoZ4NMDwYtN18obc8AemS33DBLWs3H7otXft3XjrpDtQGv7SqSsaBYBb98uNbr2VBBEt7f2wfn3RVGQBEP3A",
	},
}

// --- P2Pool types ---

type share struct {
	ID              string  `json:"id"`
	Height          uint64  `json:"height"`
	Difficulty      uint64  `json:"difficulty"`
	Shares          uint64  `json:"shares"`
	Timestamp       int64   `json:"timestamp"`
	MinerAddress    string  `json:"address"`
	WorkerName      string  `json:"worker"`
	IsUncle         *bool   `json:"uncle,omitempty"`
	SoftwareID      *uint8  `json:"software_id,omitempty"`
	SoftwareVersion *string `json:"software_version,omitempty"`
}

type foundBlock struct {
	MainHeight         uint64  `json:"height"`
	MainHash           string  `json:"hash"`
	SidechainHeight    uint64  `json:"sidechain_height"`
	Reward             uint64  `json:"reward"`
	Timestamp          int64   `json:"timestamp"`
	Effort             float64 `json:"effort"`
	CoinbasePrivateKey *string `json:"coinbase_private_key,omitempty"`
}

type poolStats struct {
	PoolStatistics poolStatistics `json:"pool_statistics"`
}

type poolStatistics struct {
	HashRate              uint64 `json:"hash_rate"`
	HashRateShort         uint64 `json:"hash_rate_short"`
	Miners                int    `json:"miners"`
	TotalHashes           uint64 `json:"total_hashes"`
	LastBlockFound        uint64 `json:"last_block_found"`
	TotalBlocks           uint64 `json:"totalBlocksFound"`
	PPLNSWindow           int    `json:"pplns_window"`
	SidechainDifficulty   uint64 `json:"sidechainDifficulty"`
	SidechainHeight       uint64 `json:"sidechainHeight"`
}

type workerInfo struct {
	Shares    uint64 `json:"shares"`
	Hashes    uint64 `json:"hashes"`
	LastShare int64  `json:"last_share"`
}

type peer struct {
	ID   string `json:"id"`
	Addr string `json:"addr"`
}

// --- Monerod JSON-RPC types ---

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  interface{} `json:"result"`
}

type blockHeader struct {
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

// --- Helpers ---

func randHash() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func randUint(max int64) uint64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Uint64()
}

func makeBlockHeader(height uint64) blockHeader {
	s.mu.RLock()
	currentHeight := s.mainHeight
	s.mu.RUnlock()

	var depth uint64
	if currentHeight > height {
		depth = currentHeight - height
	}

	return blockHeader{
		Height:       height,
		Hash:         randHash(),
		Timestamp:    uint64(time.Now().Add(-time.Duration(depth) * 2 * time.Minute).Unix()),
		Reward:       600_000_000_000 + randUint(50_000_000_000),
		Difficulty:   400_000_000_000 + randUint(50_000_000_000),
		NumTxes:      int(5 + randUint(20)),
		MajorVersion: 16,
		Nonce:        randUint(999999999),
		OrphanStatus: false,
		Depth:        depth,
	}
}

// Build a fake coinbase tx JSON for a given height with outputs to our miners.
func makeCoinbaseTxJSON(height, reward uint64) string {
	s.mu.RLock()
	miners := s.miners
	s.mu.RUnlock()

	perMiner := reward / uint64(len(miners))
	var outputs []map[string]interface{}
	for i, _ := range miners {
		amt := perMiner
		if i == len(miners)-1 {
			amt = reward - perMiner*uint64(len(miners)-1)
		}
		outputs = append(outputs, map[string]interface{}{
			"amount": amt,
			"target": map[string]interface{}{
				"tagged_key": map[string]interface{}{
					"key":      randHash()[:64],
					"view_tag": fmt.Sprintf("%02x", i),
				},
			},
		})
	}

	txJSON := map[string]interface{}{
		"version":     2,
		"unlock_time": height + 60,
		"vin": []map[string]interface{}{
			{"gen": map[string]interface{}{"height": height}},
		},
		"vout":  outputs,
		"extra": []int{1, 2, 3, 4, 5},
	}

	b, _ := json.Marshal(txJSON)
	return string(b)
}

// --- Background simulation ---

func simulate() {
	workers := []string{"rig-01", "rig-02", ""}

	// Generate initial shares so the indexer has something on first poll.
	s.mu.Lock()
	now := time.Now()
	for i := 0; i < 30; i++ {
		minerIdx := i % len(s.miners)
		isUncle := boolPtr(i%10 == 0) // ~10% uncle rate
		swID := uint8Ptr(1)           // XMRig
		swVer := stringPtr("6.21.0")
		s.shares = append(s.shares, share{
			ID:              fmt.Sprintf("share-init-%d", i),
			Height:          s.sidechainHeight + uint64(i),
			Difficulty:      200_000_000 + randUint(100_000_000),
			Shares:          1,
			Timestamp:       now.Add(-time.Duration(30-i) * 30 * time.Second).Unix(),
			MinerAddress:    s.miners[minerIdx],
			WorkerName:      workers[minerIdx%len(workers)],
			IsUncle:         isUncle,
			SoftwareID:      swID,
			SoftwareVersion: swVer,
		})
	}
	s.sidechainHeight += 30

	// Generate an initial found block so the pipeline has something to scan.
	cbKey := randHash()
	s.foundBlocks = append(s.foundBlocks, foundBlock{
		MainHeight:         s.mainHeight,
		MainHash:           randHash(),
		SidechainHeight:    s.sidechainHeight - 5,
		Reward:             615_000_000_000,
		Timestamp:          now.Add(-2 * time.Minute).Unix(),
		Effort:             92.5,
		CoinbasePrivateKey: &cbKey,
	})
	s.mu.Unlock()

	// Every 10s: add new shares. Every 60s: advance main chain. Every ~90s: find a block.
	shareTicker := time.NewTicker(10 * time.Second)
	blockTicker := time.NewTicker(15 * time.Second) // faster than real for testing
	foundTicker := time.NewTicker(90 * time.Second)

	for {
		select {
		case <-shareTicker.C:
			s.mu.Lock()
			swVersions := []string{"6.21.0", "6.20.0", "6.19.1"}
			for i := 0; i < 3+int(randUint(5)); i++ {
				minerIdx := int(randUint(int64(len(s.miners))))
				s.sidechainHeight++
				s.shares = append(s.shares, share{
					ID:              fmt.Sprintf("share-%d", s.sidechainHeight),
					Height:          s.sidechainHeight,
					Difficulty:      200_000_000 + randUint(150_000_000),
					Shares:          1,
					Timestamp:       time.Now().Unix(),
					MinerAddress:    s.miners[minerIdx],
					WorkerName:      workers[minerIdx%len(workers)],
					IsUncle:         boolPtr(randUint(10) == 0),
					SoftwareID:      uint8Ptr(uint8(randUint(3))),
					SoftwareVersion: stringPtr(swVersions[randUint(int64(len(swVersions)))]),
				})
			}
			// Keep only last 2160 shares (PPLNS window).
			if len(s.shares) > 2160 {
				s.shares = s.shares[len(s.shares)-2160:]
			}
			s.mu.Unlock()
			log.Printf("[sim] added shares, sidechain_height=%d, total_shares=%d", s.sidechainHeight, len(s.shares))

		case <-blockTicker.C:
			s.mu.Lock()
			s.mainHeight++
			s.mu.Unlock()
			log.Printf("[sim] new main chain block height=%d", s.mainHeight)

		case <-foundTicker.C:
			s.mu.Lock()
			reward := 600_000_000_000 + randUint(50_000_000_000)
			cbPrivKey := randHash()
			fb := foundBlock{
				MainHeight:         s.mainHeight,
				MainHash:           randHash(),
				SidechainHeight:    s.sidechainHeight,
				Reward:             reward,
				Timestamp:          time.Now().Unix(),
				Effort:             float64(50+randUint(150)) / 1.0,
				CoinbasePrivateKey: &cbPrivKey,
			}
			s.foundBlocks = append(s.foundBlocks, fb)
			if len(s.foundBlocks) > 100 {
				s.foundBlocks = s.foundBlocks[len(s.foundBlocks)-100:]
			}
			s.mu.Unlock()
			log.Printf("[sim] P2Pool found block! main_height=%d reward=%d", fb.MainHeight, fb.Reward)
		}
	}
}

// --- P2Pool API handlers ---

func p2poolPoolStats(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lastFound uint64
	if len(s.foundBlocks) > 0 {
		lastFound = s.foundBlocks[len(s.foundBlocks)-1].MainHeight
	}

	resp := poolStats{
		PoolStatistics: poolStatistics{
			HashRate:            52_000_000 + randUint(5_000_000),
			HashRateShort:       48_000_000 + randUint(8_000_000),
			Miners:              len(s.miners),
			TotalHashes:         999_999_999 + randUint(100_000),
			LastBlockFound:      lastFound,
			TotalBlocks:         uint64(len(s.foundBlocks)),
			PPLNSWindow:         2160,
			SidechainDifficulty: 300_000_000 + randUint(50_000_000),
			SidechainHeight:     s.sidechainHeight,
		},
	}
	writeJSON(w, resp)
}

func p2poolShares(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, s.shares)
}

func p2poolFoundBlocks(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, s.foundBlocks)
}

func p2poolWorkerStats(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]workerInfo)
	for _, m := range s.miners {
		var totalShares, totalHashes uint64
		var lastShare int64
		for _, sh := range s.shares {
			if sh.MinerAddress == m {
				totalShares += sh.Shares
				totalHashes += sh.Difficulty
				if sh.Timestamp > lastShare {
					lastShare = sh.Timestamp
				}
			}
		}
		stats[m] = workerInfo{
			Shares:    totalShares,
			Hashes:    totalHashes,
			LastShare: lastShare,
		}
	}
	writeJSON(w, stats)
}

func p2poolPeers(w http.ResponseWriter, _ *http.Request) {
	peers := []peer{
		{ID: "peer-1", Addr: "192.168.1.10:37889"},
		{ID: "peer-2", Addr: "10.0.0.5:37889"},
		{ID: "peer-3", Addr: "172.16.0.20:37889"},
	}
	writeJSON(w, peers)
}

// --- Mock CoinGecko handler ---

func coingeckoPrice(w http.ResponseWriter, _ *http.Request) {
	// Return a realistic XMR price with slight random variation.
	baseUSD := 150.0 + float64(randUint(2000))/100.0  // ~$150-170
	baseCAD := baseUSD * 1.36                           // rough USD→CAD
	resp := map[string]interface{}{
		"monero": map[string]interface{}{
			"usd": baseUSD,
			"cad": baseCAD,
		},
	}
	writeJSON(w, resp)
}

// --- Monerod JSON-RPC handler ---

func monerodRPC(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	currentHeight := s.mainHeight
	s.mu.RUnlock()

	var result interface{}

	switch req.Method {
	case "get_last_block_header":
		bh := makeBlockHeader(currentHeight)
		result = map[string]interface{}{"block_header": bh}

	case "get_block_header_by_height":
		height := extractHeight(req.Params)
		bh := makeBlockHeader(height)
		result = map[string]interface{}{"block_header": bh}

	case "get_block":
		height := extractHeight(req.Params)
		bh := makeBlockHeader(height)
		minerTxHash := randHash()

		// Store the mapping so /get_transactions can use it.
		txStore.mu.Lock()
		txStore.data[minerTxHash] = txEntry{height: height, reward: bh.Reward}
		txStore.mu.Unlock()

		result = map[string]interface{}{
			"blob":          "0e0eabc...",
			"block_header":  bh,
			"json":          fmt.Sprintf(`{"major_version":%d}`, bh.MajorVersion),
			"miner_tx_hash": minerTxHash,
		}

	default:
		writeJSON(w, rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			},
		})
		return
	}

	writeJSON(w, rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

// --- Transaction store for coinbase lookups ---

type txEntry struct {
	height uint64
	reward uint64
}

var txStore = struct {
	mu   sync.RWMutex
	data map[string]txEntry
}{data: make(map[string]txEntry)}

func monerodGetTransactions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TxHashes     []string `json:"txs_hashes"`
		DecodeAsJSON bool     `json:"decode_as_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var txs []map[string]interface{}
	var missed []string

	for _, hash := range req.TxHashes {
		txStore.mu.RLock()
		entry, ok := txStore.data[hash]
		txStore.mu.RUnlock()

		if !ok {
			// Return a generic coinbase tx for unknown hashes.
			entry = txEntry{height: s.mainHeight, reward: 615_000_000_000}
		}

		txJSON := makeCoinbaseTxJSON(entry.height, entry.reward)
		txs = append(txs, map[string]interface{}{
			"tx_hash": hash,
			"as_hex":  "01...",
			"as_json": txJSON,
			"in_pool": false,
		})
	}

	resp := map[string]interface{}{
		"status": "OK",
		"txs":    txs,
	}
	if len(missed) > 0 {
		resp["missed_tx"] = missed
	}
	writeJSON(w, resp)
}

// --- Utilities ---

func extractHeight(params interface{}) uint64 {
	if params == nil {
		return s.mainHeight
	}
	m, ok := params.(map[string]interface{})
	if !ok {
		return s.mainHeight
	}
	h, ok := m["height"]
	if !ok {
		return s.mainHeight
	}
	switch v := h.(type) {
	case float64:
		return uint64(v)
	case json.Number:
		n, _ := v.Int64()
		return uint64(n)
	}
	return s.mainHeight
}

func boolPtr(v bool) *bool       { return &v }
func uint8Ptr(v uint8) *uint8     { return &v }
func stringPtr(v string) *string  { return &v }

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func main() {
	log.Println("Starting mock P2Pool + monerod simulator")

	// Start background simulation.
	go simulate()

	// P2Pool API on :3333
	p2poolMux := http.NewServeMux()
	p2poolMux.HandleFunc("/api/pool/stats", p2poolPoolStats)
	p2poolMux.HandleFunc("/api/shares", p2poolShares)
	p2poolMux.HandleFunc("/api/found_blocks", p2poolFoundBlocks)
	p2poolMux.HandleFunc("/api/worker_stats", p2poolWorkerStats)
	p2poolMux.HandleFunc("/api/p2p/peers", p2poolPeers)
	p2poolMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[p2pool] unhandled: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	go func() {
		log.Println("P2Pool API listening on :3333")
		if err := http.ListenAndServe(":3333", p2poolMux); err != nil {
			log.Fatalf("P2Pool server failed: %v", err)
		}
	}()

	// Monerod JSON-RPC on :18081
	monerodMux := http.NewServeMux()
	monerodMux.HandleFunc("/json_rpc", monerodRPC)
	monerodMux.HandleFunc("/get_transactions", monerodGetTransactions)
	monerodMux.HandleFunc("/api/v3/simple/price", coingeckoPrice)
	monerodMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[monerod] unhandled: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	log.Println("Monerod JSON-RPC listening on :18081")
	if err := http.ListenAndServe(":18081", monerodMux); err != nil {
		log.Fatalf("Monerod server failed: %v", err)
	}
}
