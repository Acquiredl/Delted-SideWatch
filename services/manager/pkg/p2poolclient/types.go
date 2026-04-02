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
	Hashrate15m         uint64   `json:"hashrate_15m"`
	Hashrate1h          uint64   `json:"hashrate_1h"`
	Hashrate24h         uint64   `json:"hashrate_24h"`
	TotalHashes         uint64   `json:"total_hashes"`
	TotalStratumShares  uint64   `json:"total_stratum_shares"`
	SharesFound         uint64   `json:"shares_found"`
	SharesFailed        uint64   `json:"shares_failed"`
	AverageEffort       float64  `json:"average_effort"`
	CurrentEffort       float64  `json:"current_effort"`
	Connections         int      `json:"connections"`
	IncomingConnections int      `json:"incoming_connections"`
	BlockRewardSharePct float64  `json:"block_reward_share_percent"`
	RawWorkers          []string `json:"workers"`
}

// StratumWorker is a parsed representation of a worker CSV string.
// P2Pool v4.x returns workers as: "IP:port,hashrate,hashes,bestDiff,walletPrefix"
type StratumWorker struct {
	Connection    string // IP:port
	Hashrate      uint64
	TotalHashes   uint64
	BestDifficulty uint64
	WalletPrefix  string // truncated wallet address
}

// Workers parses the raw CSV worker strings into structured StratumWorker values.
func (ls *LocalStratum) Workers() []StratumWorker {
	workers := make([]StratumWorker, 0, len(ls.RawWorkers))
	for _, raw := range ls.RawWorkers {
		w := ParseWorkerCSV(raw)
		if w.Connection != "" {
			workers = append(workers, w)
		}
	}
	return workers
}

// ParseWorkerCSV parses a P2Pool worker CSV string.
// Format: "IP:port,hashrate,hashes,bestDiff,walletPrefix"
func ParseWorkerCSV(raw string) StratumWorker {
	parts := splitCSV(raw)
	var w StratumWorker
	if len(parts) >= 1 {
		w.Connection = parts[0]
	}
	if len(parts) >= 2 {
		w.Hashrate = parseUint(parts[1])
	}
	if len(parts) >= 3 {
		w.TotalHashes = parseUint(parts[2])
	}
	if len(parts) >= 4 {
		w.BestDifficulty = parseUint(parts[3])
	}
	if len(parts) >= 5 {
		w.WalletPrefix = parts[4]
	}
	return w
}

func splitCSV(s string) []string {
	var parts []string
	start := 0
	for i, c := range s {
		if c == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func parseUint(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

// LocalP2P represents the response from /local/p2p.
type LocalP2P struct {
	Connections         int      `json:"connections"`
	IncomingConnections int      `json:"incoming_connections"`
	PeerListSize        int      `json:"peer_list_size"`
	Peers               []string `json:"peers"`
}
