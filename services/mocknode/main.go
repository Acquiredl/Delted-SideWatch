// mocknode simulates P2Pool data-api and monerod APIs for local testing.
// P2Pool data-api on :3333, monerod JSON-RPC on :18081.
//
// Serves the same paths as the real P2Pool --data-api + nginx sidecar:
//   /pool/stats, /network/stats, /local/stratum, /local/p2p, /stats_mod
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
	foundBlocks     []foundBlock
	miners          []string
	workers         []worker
}

type worker struct {
	Address  string
	Name     string
	Hashrate uint64
	Hashes   uint64
}

type foundBlock struct {
	MainHeight      uint64
	MainHash        string
	SidechainHeight uint64
	Reward          uint64
	Timestamp       int64
	Effort          float64
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
	for i := range miners {
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
	workerNames := []string{"rig-01", "rig-02", ""}

	// Initialize workers.
	s.mu.Lock()
	for i, addr := range s.miners {
		s.workers = append(s.workers, worker{
			Address:  addr,
			Name:     workerNames[i%len(workerNames)],
			Hashrate: 15_000_000 + randUint(5_000_000),
			Hashes:   1_000_000 + randUint(500_000),
		})
	}

	// Generate an initial found block.
	s.foundBlocks = append(s.foundBlocks, foundBlock{
		MainHeight:      s.mainHeight,
		MainHash:        randHash(),
		SidechainHeight: s.sidechainHeight - 5,
		Reward:          615_000_000_000,
		Timestamp:       time.Now().Add(-2 * time.Minute).Unix(),
		Effort:          92.5,
	})
	s.mu.Unlock()

	// Every 10s: update worker hashrates. Every 15s: advance main chain. Every 90s: find a block.
	workerTicker := time.NewTicker(10 * time.Second)
	blockTicker := time.NewTicker(15 * time.Second)
	foundTicker := time.NewTicker(90 * time.Second)

	for {
		select {
		case <-workerTicker.C:
			s.mu.Lock()
			s.sidechainHeight += 3 + randUint(5)
			for i := range s.workers {
				s.workers[i].Hashrate = 10_000_000 + randUint(10_000_000)
				s.workers[i].Hashes += s.workers[i].Hashrate * 10
			}
			s.mu.Unlock()
			log.Printf("[sim] updated workers, sidechain_height=%d", s.sidechainHeight)

		case <-blockTicker.C:
			s.mu.Lock()
			s.mainHeight++
			s.mu.Unlock()
			log.Printf("[sim] new main chain block height=%d", s.mainHeight)

		case <-foundTicker.C:
			s.mu.Lock()
			reward := 600_000_000_000 + randUint(50_000_000_000)
			fb := foundBlock{
				MainHeight:      s.mainHeight,
				MainHash:        randHash(),
				SidechainHeight: s.sidechainHeight,
				Reward:          reward,
				Timestamp:       time.Now().Unix(),
				Effort:          float64(50+randUint(150)) / 1.0,
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

// --- P2Pool data-api handlers (matches real P2Pool --data-api format) ---

func poolStats(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lastFoundTime int64
	var lastFoundHeight uint64
	if len(s.foundBlocks) > 0 {
		last := s.foundBlocks[len(s.foundBlocks)-1]
		lastFoundHeight = last.MainHeight
		lastFoundTime = last.Timestamp
	}

	var totalHashrate uint64
	for _, w := range s.workers {
		totalHashrate += w.Hashrate
	}

	resp := map[string]interface{}{
		"pool_list": []string{"pplns"},
		"pool_statistics": map[string]interface{}{
			"hashRate":            totalHashrate,
			"miners":              len(s.miners),
			"totalHashes":         999_999_999 + randUint(100_000),
			"lastBlockFoundTime":  lastFoundTime,
			"lastBlockFound":      lastFoundHeight,
			"totalBlocksFound":    uint64(len(s.foundBlocks)),
			"pplnsWeight":         524_000_000_000 + randUint(1_000_000_000),
			"pplnsWindowSize":     2160,
			"sidechainDifficulty": 300_000_000 + randUint(50_000_000),
			"sidechainHeight":     s.sidechainHeight,
		},
	}
	writeJSON(w, resp)
}

func networkStats(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := map[string]interface{}{
		"difficulty": 400_000_000_000 + randUint(50_000_000_000),
		"hash":       randHash(),
		"height":     s.mainHeight,
		"reward":     600_000_000_000 + randUint(50_000_000_000),
		"timestamp":  time.Now().Unix(),
	}
	writeJSON(w, resp)
}

func localStratum(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalHashrate, totalHashes, sharesFound uint64
	// P2Pool v4.x returns workers as CSV strings: "IP:port,hashrate,hashes,bestDiff,walletPrefix"
	workers := make([]string, 0, len(s.workers))
	for i, wk := range s.workers {
		totalHashrate += wk.Hashrate
		totalHashes += wk.Hashes
		sharesFound += 10 + randUint(50)
		// Truncate address to ~32 chars like the real P2Pool does
		addrPrefix := wk.Address
		if len(addrPrefix) > 32 {
			addrPrefix = addrPrefix[:32]
		}
		workers = append(workers, fmt.Sprintf("192.168.1.%d:%d,%d,%d,%d,%s",
			10+i, 40000+i, wk.Hashrate, wk.Hashes, 5000+randUint(5000), addrPrefix))
	}

	resp := map[string]interface{}{
		"hashrate_15m":               totalHashrate,
		"hashrate_1h":                totalHashrate - randUint(2_000_000),
		"hashrate_24h":               totalHashrate - randUint(5_000_000),
		"total_hashes":               totalHashes,
		"total_stratum_shares":       sharesFound,
		"shares_found":               sharesFound,
		"shares_failed":              randUint(5),
		"average_effort":             85.5 + float64(randUint(300))/10.0,
		"current_effort":             50.0 + float64(randUint(1000))/10.0,
		"connections":                len(s.workers),
		"incoming_connections":       0,
		"block_reward_share_percent": float64(len(s.workers)) * 0.001,
		"workers":                    workers,
	}
	writeJSON(w, resp)
}

func localP2P(w http.ResponseWriter, _ *http.Request) {
	peers := []string{
		"O,248,125,P2Pool v4.13,13401261,65.21.227.114:37888",
		"O,248,115,GoObserver v4.9.1,13401261,89.233.207.111:37888",
		"O,248,109,P2Pool v4.13,13401261,5.9.17.234:37888",
		"I,11,223,P2Pool v4.13,13401261,88.146.114.222:33290",
	}
	resp := map[string]interface{}{
		"connections":          len(peers),
		"incoming_connections": 1,
		"peer_list_size":       50 + int(randUint(100)),
		"peers":                peers,
	}
	writeJSON(w, resp)
}

func statsMod(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalHashrate uint64
	for _, wk := range s.workers {
		totalHashrate += wk.Hashrate
	}

	resp := map[string]interface{}{
		"config": map[string]interface{}{
			"ports":               []map[string]interface{}{{"port": 3333, "tls": false}},
			"fee":                 0,
			"minPaymentThreshold": 300000000,
		},
		"network": map[string]interface{}{
			"height": s.mainHeight,
		},
		"pool": map[string]interface{}{
			"stats":      map[string]interface{}{"lastBlockFound": "0000"},
			"blocks":     []string{},
			"miners":     len(s.miners),
			"hashrate":   totalHashrate,
			"roundHashes": 999_999_999 + randUint(100_000),
		},
	}
	writeJSON(w, resp)
}

// --- Mock CoinGecko handler ---

func coingeckoPrice(w http.ResponseWriter, _ *http.Request) {
	baseUSD := 150.0 + float64(randUint(2000))/100.0
	baseCAD := baseUSD * 1.36
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

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func main() {
	log.Println("Starting mock P2Pool data-api + monerod simulator")

	go simulate()

	// P2Pool data-api on :3333 (same paths as nginx sidecar serves from --data-api)
	p2poolMux := http.NewServeMux()
	p2poolMux.HandleFunc("/pool/stats", poolStats)
	p2poolMux.HandleFunc("/network/stats", networkStats)
	p2poolMux.HandleFunc("/local/stratum", localStratum)
	p2poolMux.HandleFunc("/local/p2p", localP2P)
	p2poolMux.HandleFunc("/stats_mod", statsMod)
	p2poolMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})
	p2poolMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[p2pool] unhandled: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	go func() {
		log.Println("P2Pool data-api listening on :3333")
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
