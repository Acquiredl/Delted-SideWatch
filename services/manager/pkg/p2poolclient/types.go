package p2poolclient

// PoolStats represents the response from /pool/stats (P2Pool data-api).
type PoolStats struct {
	PoolList       []string       `json:"pool_list"`
	PoolStatistics PoolStatistics `json:"pool_statistics"`
}

// PoolStatistics contains aggregate pool metrics.
type PoolStatistics struct {
	HashRate            uint64 `json:"hashRate"`
	Miners              int    `json:"miners"`
	TotalHashes         uint64 `json:"totalHashes"`
	LastBlockFoundTime  int64  `json:"lastBlockFoundTime"`
	LastBlockFound      uint64 `json:"lastBlockFound"`
	TotalBlocksFound    uint64 `json:"totalBlocksFound"`
	PPLNSWeight         uint64 `json:"pplnsWeight"`
	PPLNSWindowSize     int    `json:"pplnsWindowSize"`
	SidechainDifficulty uint64 `json:"sidechainDifficulty"`
	SidechainHeight     uint64 `json:"sidechainHeight"`
}

// NetworkStats represents the response from /network/stats.
type NetworkStats struct {
	Difficulty uint64 `json:"difficulty"`
	Hash       string `json:"hash"`
	Height     uint64 `json:"height"`
	Reward     uint64 `json:"reward"`
	Timestamp  int64  `json:"timestamp"`
}

// LocalStratum represents the response from /local/stratum.
type LocalStratum struct {
	Hashrate15m         uint64          `json:"hashrate_15m"`
	Hashrate1h          uint64          `json:"hashrate_1h"`
	Hashrate24h         uint64          `json:"hashrate_24h"`
	TotalHashes         uint64          `json:"total_hashes"`
	TotalStratumShares  uint64          `json:"total_stratum_shares"`
	SharesFound         uint64          `json:"shares_found"`
	SharesFailed        uint64          `json:"shares_failed"`
	AverageEffort       float64         `json:"average_effort"`
	CurrentEffort       float64         `json:"current_effort"`
	Connections         int             `json:"connections"`
	IncomingConnections int             `json:"incoming_connections"`
	BlockRewardSharePct float64         `json:"block_reward_share_percent"`
	Workers             []StratumWorker `json:"workers"`
}

// StratumWorker represents one miner connected to our stratum.
// P2Pool v4.x worker entries may vary by version; fields are best-effort.
type StratumWorker struct {
	Address   string `json:"address"`
	Name      string `json:"name"`
	Hashrate  uint64 `json:"hashrate"`
	Hashes    uint64 `json:"hashes"`
	LastShare int64  `json:"last_share"`
}

// LocalP2P represents the response from /local/p2p.
type LocalP2P struct {
	Connections         int      `json:"connections"`
	IncomingConnections int      `json:"incoming_connections"`
	PeerListSize        int      `json:"peer_list_size"`
	Peers               []string `json:"peers"`
}
