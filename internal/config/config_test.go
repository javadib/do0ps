package config

import (
	"errors"
	"testing"
	"time"
)

func TestLoadValid(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a,tok-b:client-b")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error = %v", err)
	}
	if cfg.AuthTokens != "tok-a:client-a,tok-b:client-b" {
		t.Errorf("AuthTokens = %q, want the raw env value", cfg.AuthTokens)
	}
	if cfg.DatabasePath != "/tmp/test.db" {
		t.Errorf("DatabasePath = %q, want /tmp/test.db", cfg.DatabasePath)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.Workers != 8 || cfg.PollTimeout != 20*time.Minute {
		t.Errorf("tunable defaults not applied: %+v", cfg)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a")
	t.Setenv("DB_PATH", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error = %v", err)
	}
	if cfg.DatabasePath != "./data/do0ps.db" {
		t.Errorf("DatabasePath = %q, want ./data/do0ps.db", cfg.DatabasePath)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("LOG_LEVEL", "")

	_, err := Load(nil)
	if err == nil {
		t.Fatal("Load(nil) succeeded, want an error for missing MCP_AUTH_TOKENS")
	}
}

func TestLoadInvalidValue(t *testing.T) {
	t.Run("http_port", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a")
		t.Setenv("HTTP_PORT", "not-a-port")
		if _, err := Load(nil); err == nil {
			t.Fatal("Load(nil) succeeded, want an error for invalid HTTP_PORT")
		}
	})

	t.Run("log_level", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a")
		t.Setenv("HTTP_PORT", "8080")
		t.Setenv("LOG_LEVEL", "verbose")
		if _, err := Load(nil); err == nil {
			t.Fatal("Load(nil) succeeded, want an error for invalid LOG_LEVEL")
		}
	})
}

func TestNewLogger(t *testing.T) {
	if _, err := NewLogger("info"); err != nil {
		t.Fatalf("NewLogger(info) error = %v", err)
	}
	if _, err := NewLogger("bogus"); err == nil {
		t.Fatal("NewLogger(bogus) succeeded, want an error")
	}
}

func TestLoadStdioTransport(t *testing.T) {
	// No MCP_AUTH_TOKENS on purpose: stdio has no listener to guard, so
	// requiring a token there would block every bundle install.
	t.Setenv("MCP_AUTH_TOKENS", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("MCP_TRANSPORT", "")

	cfg, err := Load([]string{"--stdio"})
	if err != nil {
		t.Fatalf("Load(--stdio) error = %v", err)
	}
	if cfg.Transport != TransportStdio {
		t.Errorf("Transport = %q, want %q", cfg.Transport, TransportStdio)
	}
	// The client picks the working directory, so the job store must not be
	// relative to it.
	if cfg.DatabasePath == "./data/do0ps.db" {
		t.Errorf("DatabasePath = %q, want a per-user path", cfg.DatabasePath)
	}
}

func TestLoadStdioViaEnvironment(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "")
	t.Setenv("MCP_TRANSPORT", "stdio")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error = %v", err)
	}
	if cfg.Transport != TransportStdio {
		t.Errorf("Transport = %q, want %q", cfg.Transport, TransportStdio)
	}
}

func TestLoadDBPathOverridesStdioDefault(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "")
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("DB_PATH", "/tmp/explicit.db")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error = %v", err)
	}
	if cfg.DatabasePath != "/tmp/explicit.db" {
		t.Errorf("DatabasePath = %q, want the DB_PATH override", cfg.DatabasePath)
	}
}

func TestLoadVersionFlag(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a")

	if _, err := Load([]string{"--version"}); !errors.Is(err, ErrVersionRequested) {
		t.Errorf("Load(--version) error = %v, want ErrVersionRequested", err)
	}
}

func TestLoadKeepsTransportOnValidationError(t *testing.T) {
	// main decides where startup errors may be printed from the returned
	// transport: under stdio, stdout carries the protocol and must stay clean.
	t.Setenv("MCP_AUTH_TOKENS", "")
	t.Setenv("MCP_TRANSPORT", "stdio")
	t.Setenv("LOG_LEVEL", "nonsense")

	cfg, err := Load(nil)
	if err == nil {
		t.Fatal("Load(nil) succeeded, want an error for an invalid LOG_LEVEL")
	}
	if cfg.Transport != TransportStdio {
		t.Errorf("Transport = %q on error, want %q", cfg.Transport, TransportStdio)
	}
}
