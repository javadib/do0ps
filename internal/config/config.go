// Package config loads server configuration from environment variables and
// builds the process-wide structured logger. It is plain infrastructure glue
// with no imports of Fiber, adapters, or core; cmd/server wires it into the
// rest of the application.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting read from the environment.
type Config struct {
	// AuthTokens is the raw MCP_AUTH_TOKENS value: a comma-separated list of
	// "token:client_id[:name]" entries. It is parsed by internal/auth.
	AuthTokens string
	// DatabasePath is the SQLite file location.
	DatabasePath string
	// Addr is the host:port the HTTP server listens on.
	Addr string
	// LogLevel is one of debug, info, warn, error.
	LogLevel string

	Workers      int
	QueueDepth   int
	PollInterval time.Duration
	PollTimeout  time.Duration
	ShutdownWait time.Duration
}

// Load reads the environment and validates the required settings. A missing
// or invalid required value returns an error so the server can fail fast on
// startup instead of coming up half-configured.
func Load() (Config, error) {
	cfg := Config{
		AuthTokens:   os.Getenv("MCP_AUTH_TOKENS"),
		DatabasePath: envString("DB_PATH", "./data/do0ps.db"),
		LogLevel:     envString("LOG_LEVEL", "info"),
		Workers:      8,
		QueueDepth:   256,
		PollInterval: 10 * time.Second,
		PollTimeout:  20 * time.Minute,
		ShutdownWait: 30 * time.Second,
	}

	port, err := envInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	cfg.Addr = ":" + strconv.Itoa(port)

	if cfg.AuthTokens == "" {
		return Config{}, fmt.Errorf("MCP_AUTH_TOKENS is required: comma-separated bearer token entries like %q", "token-1:client-a,token-2:client-b")
	}
	if _, err := parseLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}

	if cfg.Workers, err = envInt("DO0PS_QUEUE_WORKERS", cfg.Workers); err != nil {
		return Config{}, err
	}
	if cfg.QueueDepth, err = envInt("DO0PS_QUEUE_DEPTH", cfg.QueueDepth); err != nil {
		return Config{}, err
	}
	if cfg.PollInterval, err = envDuration("DO0PS_POLL_INTERVAL", cfg.PollInterval); err != nil {
		return Config{}, err
	}
	if cfg.PollTimeout, err = envDuration("DO0PS_POLL_TIMEOUT", cfg.PollTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownWait, err = envDuration("DO0PS_SHUTDOWN_WAIT", cfg.ShutdownWait); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return v, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 30s, got %q", key, raw)
	}
	return v, nil
}
