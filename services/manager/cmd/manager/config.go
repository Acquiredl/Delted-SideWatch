package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the manager service.
type Config struct {
	P2PoolAPIURL   string
	P2PoolSidechain string
	MonerodURL     string
	MonerodZMQURL  string
	PostgresHost   string
	PostgresDB     string
	PostgresUser   string
	PostgresPass   string
	CoingeckoURL   string
	RedisURL       string
	APIPort        string
	MetricsPort    string
	LogLevel       string

	// Subscription settings.
	WalletRPCURL             string
	SubscriptionMinUSD       float64
	SubscriptionDurationDays int
	SubscriptionGraceHours   int
}

// LoadConfig reads configuration from environment variables (with Docker
// secrets fallback) and returns a populated Config.
func LoadConfig() Config {
	return Config{
		P2PoolAPIURL:    getEnvOrDefault("P2POOL_API_URL", "http://p2pool:3333"),
		P2PoolSidechain: getEnvOrDefault("P2POOL_SIDECHAIN", "mini"),
		MonerodURL:      getEnvOrDefault("MONEROD_URL", "http://monerod:18081"),
		MonerodZMQURL:   getEnvOrDefault("MONEROD_ZMQ_URL", "tcp://monerod:18083"),
		PostgresHost:    mustGetEnv("POSTGRES_HOST"),
		PostgresDB:      mustGetEnv("POSTGRES_DB"),
		PostgresUser:    mustGetEnv("POSTGRES_USER"),
		PostgresPass:    mustGetEnv("POSTGRES_PASSWORD"),
		CoingeckoURL:    getEnvOrDefault("COINGECKO_URL", ""),
		RedisURL:        getEnvOrDefault("REDIS_URL", "redis:6379"),
		APIPort:         getEnvOrDefault("API_PORT", "8081"),
		MetricsPort:     getEnvOrDefault("METRICS_PORT", "9090"),
		LogLevel:        getEnvOrDefault("LOG_LEVEL", "info"),

		WalletRPCURL:             getEnvOrDefault("WALLET_RPC_URL", ""),
		SubscriptionMinUSD:       getEnvFloat("SUBSCRIPTION_MIN_USD", 4.0),
		SubscriptionDurationDays: getEnvInt("SUBSCRIPTION_DURATION_DAYS", 30),
		SubscriptionGraceHours:   getEnvInt("SUBSCRIPTION_GRACE_HOURS", 48),
	}
}

// getEnvInt reads an integer env var with a default fallback.
func getEnvInt(key string, defaultVal int) int {
	v := getEnvOrDefault(key, "")
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// getEnvFloat reads a float64 env var with a default fallback.
func getEnvFloat(key string, defaultVal float64) float64 {
	v := getEnvOrDefault(key, "")
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// PostgresConnString returns a pgx-compatible connection string.
func (c Config) PostgresConnString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=disable",
		c.PostgresUser, c.PostgresPass, c.PostgresHost, c.PostgresDB,
	)
}

// mustGetEnv reads a value from Docker secrets first, then falls back to
// environment variables. It panics if the value is empty or missing.
func mustGetEnv(key string) string {
	v := readSecret(key)
	if v != "" {
		return v
	}
	v = os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

// getEnvOrDefault reads a value from Docker secrets first, then env vars,
// and falls back to the provided default.
func getEnvOrDefault(key, defaultVal string) string {
	v := readSecret(key)
	if v != "" {
		return v
	}
	v = os.Getenv(key)
	if v != "" {
		return v
	}
	return defaultVal
}

// readSecret attempts to read a Docker secret from /run/secrets/<key>.
func readSecret(key string) string {
	path := "/run/secrets/" + strings.ToLower(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
