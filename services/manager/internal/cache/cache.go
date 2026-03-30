package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store provides Redis caching operations.
type Store struct {
	client *redis.Client
	logger *slog.Logger
}

// New creates a new Redis cache store and verifies connectivity.
func New(addr string, logger *slog.Logger) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("pinging redis at %s: %w", addr, err)
	}

	logger.Info("connected to redis", slog.String("addr", addr))

	return &Store{
		client: client,
		logger: logger.With(slog.String("component", "cache")),
	}, nil
}

// Get retrieves a cached value and unmarshals it into dest.
// Returns true if the key was found, false if it was a cache miss.
func (s *Store) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("getting key %s from cache: %w", key, err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("unmarshaling cached value for key %s: %w", key, err)
	}

	return true, nil
}

// Set caches a value with the given TTL.
func (s *Store) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling value for key %s: %w", key, err)
	}

	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("setting key %s in cache: %w", key, err)
	}

	return nil
}

// Delete removes a cached value.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("deleting key %s from cache: %w", key, err)
	}
	return nil
}

// HealthCheck pings Redis and returns an error if it is unreachable.
func (s *Store) HealthCheck(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check: %w", err)
	}
	return nil
}

// Close closes the Redis client connection.
func (s *Store) Close() error {
	if err := s.client.Close(); err != nil {
		return fmt.Errorf("closing redis client: %w", err)
	}
	return nil
}
