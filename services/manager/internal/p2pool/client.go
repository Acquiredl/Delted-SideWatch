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

// Client returns the underlying P2Pool API client.
func (s *Service) Client() *p2poolclient.Client {
	return s.client
}

// FetchPoolStats retrieves aggregate pool statistics from the P2Pool data-api.
func (s *Service) FetchPoolStats(ctx context.Context) (*p2poolclient.PoolStats, error) {
	stats, err := s.client.GetPoolStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching pool stats for %s sidechain: %w", s.sidechain, err)
	}
	return stats, nil
}

// FetchNetworkStats retrieves Monero network stats from the P2Pool data-api.
func (s *Service) FetchNetworkStats(ctx context.Context) (*p2poolclient.NetworkStats, error) {
	stats, err := s.client.GetNetworkStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching network stats: %w", err)
	}
	return stats, nil
}

// FetchLocalStratum retrieves workers connected to our stratum.
func (s *Service) FetchLocalStratum(ctx context.Context) (*p2poolclient.LocalStratum, error) {
	stratum, err := s.client.GetLocalStratum(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching local stratum for %s sidechain: %w", s.sidechain, err)
	}
	s.logger.Debug("fetched local stratum", slog.Int("workers", len(stratum.Workers)))
	return stratum, nil
}

// FetchLocalP2P retrieves peer connection info.
func (s *Service) FetchLocalP2P(ctx context.Context) (*p2poolclient.LocalP2P, error) {
	p2p, err := s.client.GetLocalP2P(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching local p2p: %w", err)
	}
	return p2p, nil
}
