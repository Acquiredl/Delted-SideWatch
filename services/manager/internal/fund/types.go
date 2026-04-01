package fund

import "time"

// FundStatus is the API response for GET /api/fund/status.
type FundStatus struct {
	Month          string     `json:"month"` // e.g. "2026-04"
	GoalUSD        float64    `json:"goal_usd"`
	FundedUSD      float64    `json:"funded_usd"`
	PercentFunded  int        `json:"percent_funded"`
	InfraCostUSD   float64    `json:"infra_cost_usd"`
	SupporterCount int        `json:"supporter_count"`
	NodeCount      int        `json:"node_count"`
	Nodes          []FundNode `json:"nodes"`
}

// FundNode is a summary of a node shown alongside fund status.
type FundNode struct {
	Name      string `json:"name"`
	Sidechain string `json:"sidechain"`
	Status    string `json:"status"`
	Miners    *int   `json:"miners,omitempty"`
}

// FundMonth is a historical monthly funding record for the chart.
type FundMonth struct {
	Month          string  `json:"month"`
	GoalUSD        float64 `json:"goal_usd"`
	FundedUSD      float64 `json:"funded_usd"`
	SupporterCount int     `json:"supporter_count"`
}

// Supporter is a contributor entry for the supporters page (opt-in).
type Supporter struct {
	Address string    `json:"address"` // truncated: first 4 + last 4 chars
	Tier    string    `json:"tier"`
	Since   time.Time `json:"since"`
}

// PercentFunded calculates the integer funding percentage (0-100+).
func PercentFunded(funded, goal float64) int {
	if goal <= 0 {
		return 0
	}
	pct := int(funded / goal * 100)
	if pct < 0 {
		return 0
	}
	return pct
}

// TruncateAddress returns first 4 + "..." + last 4 characters of a miner address.
func TruncateAddress(addr string) string {
	if len(addr) <= 12 {
		return addr
	}
	return addr[:4] + "..." + addr[len(addr)-4:]
}
