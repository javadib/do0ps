// Package config loads server configuration from environment variables and
// builds the process-wide structured logger. It is plain infrastructure glue
// with no imports of Fiber, adapters, or core; cmd/server wires it into the
// rest of the application.
package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Transport selects how MCP clients reach this server.
type Transport string

const (
	// TransportHTTP serves Streamable HTTP over Fiber: the self-hosted
	// deployment (Docker, VPS), reachable over the network and therefore
	// guarded by the bearer-token allow-list.
	TransportHTTP Transport = "http"

	// TransportStdio speaks JSON-RPC over the process's own pipes: the mode an
	// installed MCP bundle (.mcpb) runs in, where the chat client spawns this
	// binary itself. No listener, no tokens.
	TransportStdio Transport = "stdio"
)

// ErrVersionRequested is returned for --version, which prints the build
// version and exits successfully rather than starting anything.
var ErrVersionRequested = errors.New("version requested")

// Config holds every runtime setting read from the environment.
type Config struct {
	// Transport is how MCP clients reach this process: HTTP when self-hosted,
	// stdio when installed as an MCP bundle.
	Transport Transport

	// RemoteURL, when set, turns the stdio transport into a thin bridge to a
	// self-hosted do0ps server at that URL instead of running the adapters in
	// this process. It is what an installed bundle fills in from the user's
	// extension settings, so a team can share one server.
	RemoteURL string
	// RemoteToken is the bearer token presented to RemoteURL. Required
	// whenever RemoteURL is set, ignored otherwise.
	RemoteToken string
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
//
// args are the process arguments after the program name (pass nil when there
// are none). Only the transport is selectable there: an MCP bundle manifest
// configures a command line, not a container environment, so --stdio has to be
// expressible as a flag. Everything else stays environment-only.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("do0ps", flag.ContinueOnError)
	stdio := fs.Bool("stdio", false, "serve MCP over stdio instead of HTTP (used by installed MCP bundles)")
	showVersion := fs.Bool("version", false, "print the build version and exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parsing flags: %w", err)
	}
	if *showVersion {
		return Config{}, ErrVersionRequested
	}

	// A .env file is a local-development convenience, not a requirement:
	// containers and CI supply the environment directly, and the image
	// deliberately ships without one (.env is in .dockerignore). Treat a
	// missing file as normal and let the checks below decide whether the
	// resulting configuration is usable.
	_ = godotenv.Load()

	cfg := Config{
		Transport:    TransportHTTP,
		AuthTokens:   os.Getenv("MCP_AUTH_TOKENS"),
		LogLevel:     envString("LOG_LEVEL", "info"),
		Workers:      8,
		QueueDepth:   256,
		PollInterval: 10 * time.Second,
		PollTimeout:  20 * time.Minute,
		ShutdownWait: 30 * time.Second,
	}

	// The flag wins over the environment variable; both exist because bundle
	// manifests and container runtimes prefer different mechanisms.
	if *stdio || os.Getenv("MCP_TRANSPORT") == string(TransportStdio) {
		cfg.Transport = TransportStdio
	}
	cfg.DatabasePath = databasePath(cfg.Transport)
	cfg.RemoteURL = userConfigValue(os.Getenv("DO0PS_SERVER_URL"))
	cfg.RemoteToken = userConfigValue(os.Getenv("DO0PS_AUTH_TOKEN"))

	if cfg.RemoteURL != "" {
		if cfg.Transport != TransportStdio {
			return cfg, errors.New("DO0PS_SERVER_URL only applies to the stdio transport: it points this process at another do0ps server, which is not something a server does for itself")
		}
		if cfg.RemoteToken == "" {
			return cfg, errors.New("DO0PS_AUTH_TOKEN is required alongside DO0PS_SERVER_URL: the remote server's MCP endpoint is behind a bearer allow-list")
		}
	}

	port, err := envInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	cfg.Addr = ":" + strconv.Itoa(port)

	// Only the HTTP transport is reachable over the network, so only it needs
	// the allow-list. Requiring tokens under stdio would make every bundle
	// install ask the user for a secret that guards nothing: there the OS
	// process boundary is the trust boundary.
	if cfg.Transport == TransportHTTP && cfg.AuthTokens == "" {
		return cfg, fmt.Errorf("MCP_AUTH_TOKENS is required: comma-separated bearer token entries like %q", "token-1:client-a,token-2:client-b")
	}
	if _, err := parseLevel(cfg.LogLevel); err != nil {
		return cfg, err
	}

	if cfg.Workers, err = envInt("DO0PS_QUEUE_WORKERS", cfg.Workers); err != nil {
		return cfg, err
	}
	if cfg.QueueDepth, err = envInt("DO0PS_QUEUE_DEPTH", cfg.QueueDepth); err != nil {
		return cfg, err
	}
	if cfg.PollInterval, err = envDuration("DO0PS_POLL_INTERVAL", cfg.PollInterval); err != nil {
		return cfg, err
	}
	if cfg.PollTimeout, err = envDuration("DO0PS_POLL_TIMEOUT", cfg.PollTimeout); err != nil {
		return cfg, err
	}
	if cfg.ShutdownWait, err = envDuration("DO0PS_SHUTDOWN_WAIT", cfg.ShutdownWait); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// databasePath decides where the job store lives.
//
// Under HTTP the process is deployed deliberately -- a container with a mounted
// volume, or a chosen working directory -- so the relative default is right.
// Under stdio the chat client spawns the binary with a working directory
// nobody chose (often read-only, or the client's own install directory), so the
// job store goes to the per-user config directory instead. DB_PATH overrides
// both.
func databasePath(t Transport) string {
	if path := os.Getenv("DB_PATH"); path != "" {
		return path
	}
	if t != TransportStdio {
		return "./data/do0ps.db"
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		// No usable config directory (an unset HOME, typically). The relative
		// default is still better than failing to start: the job store is
		// recoverable state, not a hard requirement for answering a tool call.
		return "./data/do0ps.db"
	}
	return filepath.Join(dir, "do0ps", "jobs.db")
}

// userConfigValue cleans a value that came from an MCP bundle's user_config.
//
// A host app substitutes ${user_config.x} into the environment it spawns the
// bundle with. When the user leaves an optional field empty, some hosts pass
// the placeholder through literally rather than an empty string — which would
// otherwise be read as a configured value and fail with a baffling error.
func userConfigValue(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return ""
	}
	return value
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
