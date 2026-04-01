package nodepool

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/metrics"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/p2poolclient"
)

// Pool manages the shared P2Pool node pool — health checks, connection info,
// and node assignment for subscribers.
type Pool struct {
	db           *pgxpool.Pool
	stratumHost  string // public hostname for stratum URLs
	onionURL     string // optional .onion stratum URL
	pollInterval time.Duration
	logger       *slog.Logger
}

// Config holds node pool configuration.
type Config struct {
	StratumHost  string        // e.g. "sidewatch.example.com"
	OnionURL     string        // e.g. "sidewatch...onion:3333"
	PollInterval time.Duration // health check interval (default 60s)
}

// New creates a new node pool manager.
func New(db *pgxpool.Pool, cfg Config, logger *slog.Logger) *Pool {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}
	return &Pool{
		db:           db,
		stratumHost:  cfg.StratumHost,
		onionURL:     cfg.OnionURL,
		pollInterval: cfg.PollInterval,
		logger:       logger.With(slog.String("component", "nodepool")),
	}
}

// RunHealthChecker starts a background loop that polls each running node's
// P2Pool API and records health results. Blocks until ctx is cancelled.
func (p *Pool) RunHealthChecker(ctx context.Context) error {
	p.logger.Info("node health checker starting", slog.Duration("interval", p.pollInterval))

	// Initial check.
	p.checkAllNodes(ctx)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("node health checker stopped")
			return nil
		case <-ticker.C:
			p.checkAllNodes(ctx)
		}
	}
}

// checkAllNodes fetches all running nodes and checks each one.
func (p *Pool) checkAllNodes(ctx context.Context) {
	nodes, err := p.getRunningNodes(ctx)
	if err != nil {
		p.logger.Error("failed to fetch running nodes", slog.String("error", err.Error()))
		return
	}

	for _, node := range nodes {
		p.checkNode(ctx, node)
	}
}

// checkNode pings a node's P2Pool API and records the result.
func (p *Pool) checkNode(ctx context.Context, node NodePoolEntry) {
	if node.APIURL == nil || *node.APIURL == "" {
		p.logger.Warn("node has no API URL, skipping health check",
			slog.Int64("node_id", node.ID), slog.String("name", node.Name))
		return
	}

	client := p2poolclient.New(*node.APIURL, p.logger)

	stats, err := client.GetPoolStats(ctx)

	var health HealthStatus
	var hashrate *int64
	var peers *int
	var miners *int

	if err != nil {
		health = HealthUnhealthy
		p.logger.Warn("node health check failed",
			slog.String("name", node.Name),
			slog.String("error", err.Error()),
		)
	} else {
		health = HealthHealthy
		hr := int64(stats.PoolStatistics.HashRate)
		hashrate = &hr
		m := stats.PoolStatistics.Miners
		miners = &m
	}

	// Also fetch peer count (best-effort).
	if err == nil {
		peerList, peerErr := client.GetPeers(ctx)
		if peerErr == nil {
			pc := len(peerList)
			peers = &pc
		}
	}

	// Record health log.
	if logErr := p.recordHealthLog(ctx, node.ID, health, hashrate, peers, miners); logErr != nil {
		p.logger.Error("failed to record health log",
			slog.String("name", node.Name),
			slog.String("error", logErr.Error()),
		)
	}

	// Update node_pool row.
	if updateErr := p.updateNodeHealth(ctx, node.ID, health); updateErr != nil {
		p.logger.Error("failed to update node health",
			slog.String("name", node.Name),
			slog.String("error", updateErr.Error()),
		)
	}

	// Update Prometheus metrics.
	metrics.NodeHealthStatus.WithLabelValues(node.Name, node.Sidechain).Set(healthToFloat(health))
	if hashrate != nil {
		metrics.NodeHashrate.WithLabelValues(node.Name, node.Sidechain).Set(float64(*hashrate))
	}
	if miners != nil {
		metrics.NodeMiners.WithLabelValues(node.Name, node.Sidechain).Set(float64(*miners))
	}
}

// GetNodesStatus returns the public health summary of all nodes.
func (p *Pool) GetNodesStatus(ctx context.Context) (*NodeStatusResponse, error) {
	rows, err := p.db.Query(ctx,
		`SELECT name, sidechain, health_status, last_health_at
		 FROM node_pool
		 WHERE status != 'stopped'
		 ORDER BY sidechain, name`)
	if err != nil {
		return nil, fmt.Errorf("querying node pool: %w", err)
	}
	defer rows.Close()

	var nodes []NodeSummary
	for rows.Next() {
		var n NodeSummary
		var healthStr string
		if err := rows.Scan(&n.Name, &n.Sidechain, &healthStr, &n.LastHealthAt); err != nil {
			return nil, fmt.Errorf("scanning node pool row: %w", err)
		}
		n.Status = HealthStatus(healthStr)

		// Fetch latest health log for this node to get live stats.
		var logID int64
		err := p.db.QueryRow(ctx,
			`SELECT nhl.id, nhl.hashrate, nhl.peers, nhl.miners
			 FROM node_health_log nhl
			 JOIN node_pool np ON np.id = nhl.node_pool_id
			 WHERE np.name = $1
			 ORDER BY nhl.created_at DESC LIMIT 1`, n.Name,
		).Scan(&logID, &n.Hashrate, &n.Peers, &n.Miners)
		if err != nil {
			// No health log yet — that's fine.
			p.logger.Debug("no health log for node", slog.String("name", n.Name))
		}

		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating node pool rows: %w", err)
	}

	return &NodeStatusResponse{Nodes: nodes}, nil
}

// GetConnectionInfo returns stratum URLs and XMRig configs for all running nodes.
func (p *Pool) GetConnectionInfo(ctx context.Context) (*ConnectionInfoResponse, error) {
	rows, err := p.db.Query(ctx,
		`SELECT name, sidechain, status, stratum_port, health_status
		 FROM node_pool
		 WHERE status = 'running'
		 ORDER BY sidechain, name`)
	if err != nil {
		return nil, fmt.Errorf("querying node pool for connection info: %w", err)
	}
	defer rows.Close()

	var nodes []NodeConnectionInfo
	for rows.Next() {
		var name, sidechain, statusStr, healthStr string
		var stratumPort *int
		if err := rows.Scan(&name, &sidechain, &statusStr, &stratumPort, &healthStr); err != nil {
			return nil, fmt.Errorf("scanning connection info row: %w", err)
		}

		if stratumPort == nil {
			continue // No port configured — skip.
		}

		stratumURL := fmt.Sprintf("%s:%d", p.stratumHost, *stratumPort)

		nodes = append(nodes, NodeConnectionInfo{
			Name:       name,
			Sidechain:  sidechain,
			Status:     healthStr,
			StratumURL: stratumURL,
			XMRigConfig: XMRigConf{
				URL:  stratumURL,
				User: "YOUR_WALLET_ADDRESS",
				Pass: "x",
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating connection info rows: %w", err)
	}

	return &ConnectionInfoResponse{
		Nodes:    nodes,
		OnionURL: p.onionURL,
	}, nil
}

// AssignNode atomically assigns a subscriber to the least-loaded running node
// for the given sidechain. Uses FOR UPDATE SKIP LOCKED to avoid contention.
// Returns the node ID, or 0 if no running node exists for that sidechain.
func (p *Pool) AssignNode(ctx context.Context, sidechain string) (int64, error) {
	var nodeID int64
	err := p.db.QueryRow(ctx,
		`UPDATE node_pool SET subscriber_count = subscriber_count + 1
		 WHERE id = (
		     SELECT id FROM node_pool
		     WHERE sidechain = $1 AND status = 'running'
		     ORDER BY subscriber_count ASC
		     LIMIT 1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id`,
		sidechain,
	).Scan(&nodeID)
	if err != nil {
		return 0, fmt.Errorf("assigning node for sidechain %s: %w", sidechain, err)
	}

	return nodeID, nil
}

// getRunningNodes returns all nodes with status 'running'.
func (p *Pool) getRunningNodes(ctx context.Context) ([]NodePoolEntry, error) {
	rows, err := p.db.Query(ctx,
		`SELECT id, name, sidechain, status, stratum_host, stratum_port, api_url, p2p_port,
		        subscriber_count, last_health_at, health_status, created_at
		 FROM node_pool
		 WHERE status = 'running'`)
	if err != nil {
		return nil, fmt.Errorf("querying running nodes: %w", err)
	}
	defer rows.Close()

	var nodes []NodePoolEntry
	for rows.Next() {
		var n NodePoolEntry
		var statusStr, healthStr string
		if err := rows.Scan(&n.ID, &n.Name, &n.Sidechain, &statusStr, &n.StratumHost,
			&n.StratumPort, &n.APIURL, &n.P2PPort, &n.SubscriberCount,
			&n.LastHealthAt, &healthStr, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning node pool row: %w", err)
		}
		n.Status = NodeStatus(statusStr)
		n.HealthStatus = HealthStatus(healthStr)
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// recordHealthLog inserts a health check result.
func (p *Pool) recordHealthLog(ctx context.Context, nodePoolID int64, status HealthStatus, hashrate *int64, peers, miners *int) error {
	_, err := p.db.Exec(ctx,
		`INSERT INTO node_health_log (node_pool_id, status, hashrate, peers, miners)
		 VALUES ($1, $2, $3, $4, $5)`,
		nodePoolID, string(status), hashrate, peers, miners,
	)
	return err
}

// updateNodeHealth updates the node_pool row with the latest health status.
func (p *Pool) updateNodeHealth(ctx context.Context, nodePoolID int64, status HealthStatus) error {
	_, err := p.db.Exec(ctx,
		`UPDATE node_pool SET health_status = $1, last_health_at = NOW() WHERE id = $2`,
		string(status), nodePoolID,
	)
	return err
}

// healthToFloat converts HealthStatus to a float for Prometheus (1=healthy, 0=unhealthy).
func healthToFloat(h HealthStatus) float64 {
	if h == HealthHealthy {
		return 1
	}
	return 0
}
