package nodepool

import "time"

// NodeStatus represents the operational state of a shared P2Pool node.
type NodeStatus string

const (
	StatusRunning  NodeStatus = "running"
	StatusStopped  NodeStatus = "stopped"
	StatusSyncing  NodeStatus = "syncing"
	StatusDegraded NodeStatus = "degraded"
)

// HealthStatus represents the result of a node health check.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthSyncing   HealthStatus = "syncing"
	HealthUnknown   HealthStatus = "unknown"
)

// NodePoolEntry is a shared P2Pool node in the pool (maps to node_pool table).
type NodePoolEntry struct {
	ID              int64        `json:"id"`
	Name            string       `json:"name"`
	Sidechain       string       `json:"sidechain"`
	Status          NodeStatus   `json:"status"`
	StratumHost     *string      `json:"stratum_host,omitempty"`
	StratumPort     *int         `json:"stratum_port,omitempty"`
	APIURL          *string      `json:"-"` // internal only, not exposed
	P2PPort         *int         `json:"p2p_port,omitempty"`
	SubscriberCount int          `json:"subscriber_count"`
	LastHealthAt    *time.Time   `json:"last_health_at,omitempty"`
	HealthStatus    HealthStatus `json:"health_status"`
	CreatedAt       time.Time    `json:"created_at"`
}

// NodeHealthLog is a single health check result (maps to node_health_log table).
type NodeHealthLog struct {
	ID         int64        `json:"id"`
	NodePoolID int64        `json:"node_pool_id"`
	Status     HealthStatus `json:"status"`
	Hashrate   *int64       `json:"hashrate,omitempty"`
	Peers      *int         `json:"peers,omitempty"`
	Miners     *int         `json:"miners,omitempty"`
	UptimeSecs *int64       `json:"uptime_secs,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// NodeStatusResponse is the API response for GET /api/nodes/status.
type NodeStatusResponse struct {
	Nodes []NodeSummary `json:"nodes"`
}

// NodeSummary is the public-facing summary of a shared node.
type NodeSummary struct {
	Name         string       `json:"name"`
	Sidechain    string       `json:"sidechain"`
	Status       HealthStatus `json:"status"`
	Miners       *int         `json:"miners,omitempty"`
	Hashrate     *int64       `json:"hashrate,omitempty"`
	Peers        *int         `json:"peers,omitempty"`
	LastHealthAt *time.Time   `json:"last_health_at,omitempty"`
}

// ConnectionInfoResponse is the API response for GET /api/nodes/connection-info.
type ConnectionInfoResponse struct {
	Nodes    []NodeConnectionInfo `json:"nodes"`
	OnionURL string               `json:"onion_url,omitempty"`
}

// NodeConnectionInfo provides stratum URL and XMRig config for a node.
type NodeConnectionInfo struct {
	Name        string    `json:"name"`
	Sidechain   string    `json:"sidechain"`
	Status      string    `json:"status"`
	StratumURL  string    `json:"stratum_url"`
	XMRigConfig XMRigConf `json:"xmrig_config"`
}

// XMRigConf is a minimal XMRig pool configuration block.
type XMRigConf struct {
	URL  string `json:"url"`
	User string `json:"user"`
	Pass string `json:"pass"`
}
