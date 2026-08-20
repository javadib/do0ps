package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// parseLevel maps a LOG_LEVEL value onto an slog.Level. The accepted values
// match slog's own names (debug, info, warn, error), case-insensitive.
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL %q is invalid: want one of debug, info, warn, error", s)
	}
}

// NewLogger builds the process-wide structured logger: JSON output to stdout
// at the given level. Every package that needs to log should receive the
// logger returned here.
func NewLogger(level string) (*slog.Logger, error) {
	return NewLoggerTo(level, os.Stdout)
}

// NewLoggerTo is NewLogger with an explicit sink.
//
// It exists for the stdio transport, where stdout is the JSON-RPC channel: a
// single log line written there corrupts the stream and the client drops the
// connection, so those logs go to stderr instead (chat clients capture it into
// their own log files).
func NewLoggerTo(level string, out io.Writer) (*slog.Logger, error) {
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	return slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{Level: lvl})), nil
}
