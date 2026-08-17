package config

import (
	"testing"
	"time"
)

func TestLoadValid(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a,tok-b:client-b")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
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

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
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

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded, want an error for missing MCP_AUTH_TOKENS")
	}
}

func TestLoadInvalidValue(t *testing.T) {
	t.Run("http_port", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a")
		t.Setenv("HTTP_PORT", "not-a-port")
		if _, err := Load(); err == nil {
			t.Fatal("Load() succeeded, want an error for invalid HTTP_PORT")
		}
	})

	t.Run("log_level", func(t *testing.T) {
		t.Setenv("MCP_AUTH_TOKENS", "tok-a:client-a")
		t.Setenv("HTTP_PORT", "8080")
		t.Setenv("LOG_LEVEL", "verbose")
		if _, err := Load(); err == nil {
			t.Fatal("Load() succeeded, want an error for invalid LOG_LEVEL")
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
