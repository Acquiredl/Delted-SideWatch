package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration for the gateway service.
type Config struct {
	ManagerURL   string
	APIPort      string
	JWTSecret    string
	RedisURL     string
	RateLimitRPS string
	LogLevel     string
}

// LoadConfig reads configuration from environment variables (with Docker
// secrets fallback) and returns a populated Config.
func LoadConfig() Config {
	return Config{
		ManagerURL:   getEnvOrDefault("MANAGER_URL", "http://manager:8081"),
		APIPort:      getEnvOrDefault("GATEWAY_PORT", "8080"),
		JWTSecret:    mustGetEnv("JWT_SECRET"),
		RedisURL:     getEnvOrDefault("REDIS_URL", "redis:6379"),
		RateLimitRPS: getEnvOrDefault("RATE_LIMIT_RPS", "10"),
		LogLevel:     getEnvOrDefault("LOG_LEVEL", "info"),
	}
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
