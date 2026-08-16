package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// config is read from the environment (12-factor): no config file, no flags
// beyond what a container needs.
type config struct {
	Addr         string
	DatabasePath string
	Tokens       string
	Workers      int
	QueueDepth   int
	PollInterval time.Duration
	PollTimeout  time.Duration
	ShutdownWait time.Duration
}

func loadConfig() (config, error) {
	cfg := config{
		Addr:         envString("DO0PS_ADDR", ":8080"),
		DatabasePath: envString("DO0PS_DB_PATH", "do0ps.db"),
		Tokens:       os.Getenv("DO0PS_TOKENS"),
		Workers:      8,
		QueueDepth:   256,
		PollInterval: 10 * time.Second,
		PollTimeout:  20 * time.Minute,
		ShutdownWait: 30 * time.Second,
	}

	if cfg.Tokens == "" {
		return config{}, fmt.Errorf(`DO0PS_TOKENS is required, e.g. DO0PS_TOKENS="<token>:client-a"`)
	}

	var err error
	if cfg.Workers, err = envInt("DO0PS_QUEUE_WORKERS", cfg.Workers); err != nil {
		return config{}, err
	}
	if cfg.QueueDepth, err = envInt("DO0PS_QUEUE_DEPTH", cfg.QueueDepth); err != nil {
		return config{}, err
	}
	if cfg.PollInterval, err = envDuration("DO0PS_POLL_INTERVAL", cfg.PollInterval); err != nil {
		return config{}, err
	}
	if cfg.PollTimeout, err = envDuration("DO0PS_POLL_TIMEOUT", cfg.PollTimeout); err != nil {
		return config{}, err
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
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
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
		return 0, fmt.Errorf("%s must be a duration such as 30s: %w", key, err)
	}
	return v, nil
}
