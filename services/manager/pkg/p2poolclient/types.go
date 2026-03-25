package p2poolclient

// PoolStats represents the response from GET /api/pool/stats.
type PoolStats struct {
	PoolStatistics PoolStatistics `json:"pool_statistics"`
}

// PoolStatistics contains aggregate pool metrics.
type PoolStatistics struct {
	HashRate            uint64 `json:"hash_rate"`
	HashRateShort       uint64 `json:"hash_rate_short"` // 15-min window
	Miners              int    `json:"miners"`
	TotalHashes         uint64 `json:"total_hashes"`
	LastBlockFound      uint64 `json:"last_block_found"`
	TotalBlocks         uint64 `json:"totalBlocksFound"`
	PPLNSWindow         int    `json:"pplns_window"`
	SidechainDifficulty uint64 `json:"sidechainDifficulty"`
	SidechainHeight     uint64 `json:"sidechainHeight"`
}

// Share represents one entry from GET /api/shares (current PPLNS window).
type Share struct {
	ID           string `json:"id"`
	Height       uint64 `json:"height"`
	Difficulty   uint64 `json:"difficulty"`
	Shares       uint64 `json:"shares"`
	Timestamp    int64  `json:"timestamp"`
	MinerAddress string `json:"address"`
	WorkerName   string `json:"worker"`
}

// FoundBlock represents one entry from GET /api/found_blocks.
type FoundBlock struct {
	MainHeight      uint64  `json:"height"`
	MainHash        string  `json:"hash"`
	SidechainHeight uint64  `json:"sidechain_height"`
	Reward          uint64  `json:"reward"`
	Timestamp       int64   `json:"timestamp"`
	Effort          float64 `json:"effort"`
}

// WorkerStats maps miner address to their stats from GET /api/worker_stats.
type WorkerStats map[string]WorkerInfo

// WorkerInfo contains per-miner statistics.
type WorkerInfo struct {
	Shares    uint64 `json:"shares"`
	Hashes    uint64 `json:"hashes"`
	LastShare int64  `json:"last_share"`
}

// Peer represents a P2P peer from GET /api/p2p/peers.
type Peer struct {
	ID   string `json:"id"`
	Addr string `json:"addr"`
}
