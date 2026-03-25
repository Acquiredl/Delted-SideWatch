package p2pool

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/p2poolclient"
)

// Service wraps the P2Pool API client and adds sidechain awareness.
type Service struct {
	client    *p2poolclient.Client
	sidechain string // "mini" or "main"
	logger    *slog.Logger
}

// NewService creates a new P2Pool service wrapper.
func NewService(client *p2poolclient.Client, sidechain string, logger *slog.Logger) *Service {
	return &Service{
		client:    client,
		sidechain: sidechain,
		logger:    logger.With(slog.String("component", "p2pool-service"), slog.String("sidechain", sidechain)),
	}
}

// Sidechain returns the configured sidechain identifier ("mini" or "main").
func (s *Service) Sidechain() string {
	return s.sidechain
}

// FetchShares retrieves the current PPLNS window shares from the P2Pool API.
func (s *Service) FetchShares(ctx context.Context) ([]p2poolclient.Share, error) {
	shares, err := s.client.GetShares(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching shares for %s sidechain: %w", s.sidechain, err)
	}
	s.logger.Debug("fetched shares", slog.Int("count", len(shares)))
	return shares, nil
}

// FetchFoundBlocks retrieves found blocks from the P2Pool API.
func (s *Service) FetchFoundBlocks(ctx context.Context) ([]p2poolclient.FoundBlock, error) {
	blocks, err := s.client.GetFoundBlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching found blocks for %s sidechain: %w", s.sidechain, err)
	}
	s.logger.Debug("fetched found blocks", slog.Int("count", len(blocks)))
	return blocks, nil
}

// FetchPoolStats retrieves aggregate pool statistics from the P2Pool API.
func (s *Service) FetchPoolStats(ctx context.Context) (*p2poolclient.PoolStats, error) {
	stats, err := s.client.GetPoolStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching pool stats for %s sidechain: %w", s.sidechain, err)
	}
	return stats, nil
}

// FetchWorkerStats retrieves per-miner statistics from the P2Pool API.
func (s *Service) FetchWorkerStats(ctx context.Context) (p2poolclient.WorkerStats, error) {
	stats, err := s.client.GetWorkerStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching worker stats for %s sidechain: %w", s.sidechain, err)
	}
	return stats, nil
}
