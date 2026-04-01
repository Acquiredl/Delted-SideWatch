package fund

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides fund status, history, and supporter list queries.
type Service struct {
	db           *pgxpool.Pool
	goalUSD      float64
	infraCostUSD float64
	logger       *slog.Logger
}

// NewService creates a new fund service.
func NewService(db *pgxpool.Pool, goalUSD, infraCostUSD float64, logger *slog.Logger) *Service {
	return &Service{
		db:           db,
		goalUSD:      goalUSD,
		infraCostUSD: infraCostUSD,
		logger:       logger.With(slog.String("component", "fund")),
	}
}

// GetStatus returns the current month's funding state.
func (s *Service) GetStatus(ctx context.Context) (*FundStatus, error) {
	now := time.Now().UTC()
	monthStr := now.Format("2006-01")

	// Sum confirmed payments this month, converted to USD at payment time.
	var fundedUSD float64
	var supporterCount int
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(
		     (amount::NUMERIC / 1e12) * COALESCE(xmr_usd_price, 0)
		 ), 0),
		 COUNT(DISTINCT miner_address)
		 FROM subscription_payments
		 WHERE confirmed = TRUE
		   AND created_at >= date_trunc('month', NOW())
		   AND created_at < date_trunc('month', NOW()) + INTERVAL '1 month'`,
	).Scan(&fundedUSD, &supporterCount)
	if err != nil {
		return nil, fmt.Errorf("querying fund status: %w", err)
	}

	// Get running node count and summaries.
	nodes, err := s.getRunningNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying running nodes for fund: %w", err)
	}

	return &FundStatus{
		Month:          monthStr,
		GoalUSD:        s.goalUSD,
		FundedUSD:      fundedUSD,
		PercentFunded:  PercentFunded(fundedUSD, s.goalUSD),
		InfraCostUSD:   s.infraCostUSD,
		SupporterCount: supporterCount,
		NodeCount:      len(nodes),
		Nodes:          nodes,
	}, nil
}

// GetHistory returns monthly funding records for the chart.
// Returns up to the last 12 months.
func (s *Service) GetHistory(ctx context.Context) ([]FundMonth, error) {
	rows, err := s.db.Query(ctx,
		`SELECT to_char(nfm.month, 'YYYY-MM'), nfm.goal_usd, nfm.total_funded_usd,
		        (SELECT COUNT(DISTINCT sp.miner_address)
		         FROM subscription_payments sp
		         WHERE sp.confirmed = TRUE
		           AND sp.created_at >= nfm.month
		           AND sp.created_at < nfm.month + INTERVAL '1 month')
		 FROM node_fund_months nfm
		 ORDER BY nfm.month DESC
		 LIMIT 12`)
	if err != nil {
		return nil, fmt.Errorf("querying fund history: %w", err)
	}
	defer rows.Close()

	var months []FundMonth
	for rows.Next() {
		var m FundMonth
		if err := rows.Scan(&m.Month, &m.GoalUSD, &m.FundedUSD, &m.SupporterCount); err != nil {
			return nil, fmt.Errorf("scanning fund month row: %w", err)
		}
		months = append(months, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating fund month rows: %w", err)
	}

	return months, nil
}

// GetSupporters returns the opt-in list of current month's contributors.
// Only shows subscribers who have NOT opted out (opt_out_supporters defaults false).
// Addresses are truncated to first 4 + last 4 characters.
func (s *Service) GetSupporters(ctx context.Context) ([]Supporter, error) {
	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT s.miner_address, s.tier, s.created_at
		 FROM subscription_payments sp
		 JOIN subscriptions s ON s.miner_address = sp.miner_address
		 WHERE sp.confirmed = TRUE
		   AND sp.created_at >= date_trunc('month', NOW())
		   AND sp.created_at < date_trunc('month', NOW()) + INTERVAL '1 month'
		   AND s.tier IN ('supporter', 'champion')
		 ORDER BY s.tier DESC, s.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("querying supporters: %w", err)
	}
	defer rows.Close()

	var supporters []Supporter
	for rows.Next() {
		var addr, tier string
		var since time.Time
		if err := rows.Scan(&addr, &tier, &since); err != nil {
			return nil, fmt.Errorf("scanning supporter row: %w", err)
		}
		supporters = append(supporters, Supporter{
			Address: TruncateAddress(addr),
			Tier:    tier,
			Since:   since,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating supporter rows: %w", err)
	}

	return supporters, nil
}

// getRunningNodes returns summaries of running nodes for the fund status widget.
func (s *Service) getRunningNodes(ctx context.Context) ([]FundNode, error) {
	rows, err := s.db.Query(ctx,
		`SELECT np.name, np.sidechain, np.health_status,
		        (SELECT nhl.miners FROM node_health_log nhl
		         WHERE nhl.node_pool_id = np.id
		         ORDER BY nhl.created_at DESC LIMIT 1)
		 FROM node_pool np
		 WHERE np.status = 'running'
		 ORDER BY np.sidechain, np.name`)
	if err != nil {
		return nil, fmt.Errorf("querying running nodes: %w", err)
	}
	defer rows.Close()

	var nodes []FundNode
	for rows.Next() {
		var n FundNode
		if err := rows.Scan(&n.Name, &n.Sidechain, &n.Status, &n.Miners); err != nil {
			return nil, fmt.Errorf("scanning fund node row: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
