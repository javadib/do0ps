package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/config"
)

// TestRunEndToEnd drives the composition root exactly the way cmd/server's
// main does: every adapter built and wired through its port, Fiber serving
// real HTTP, then a shutdown triggered the same way SIGTERM would be. This
// is issue #20's acceptance criteria in test form — a working MCP server
// reachable end-to-end, and a clean shutdown that doesn't lose job state.
func TestRunEndToEnd(t *testing.T) {
	cfg := config.Config{
		AuthTokens:   "smoke-test-token:smoke-client",
		DatabasePath: filepath.Join(t.TempDir(), "do0ps.db"),
		Addr:         "127.0.0.1:0",
		LogLevel:     "error",
		Workers:      2,
		QueueDepth:   8,
		PollInterval: 20 * time.Millisecond,
		PollTimeout:  time.Second,
		ShutdownWait: 5 * time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addrCh := make(chan net.Addr, 1)
	runErr := make(chan error, 1)
	go func() { runErr <- run(ctx, cfg, logger, func(a net.Addr) { addrCh <- a }) }()

	base := "http://" + waitListening(t, addrCh)

	// /healthz stays outside the allow-list on purpose, for orchestrator probes.
	waitFor200(t, base+"/healthz")

	// An MCP call without a token never reaches the tool registry.
	resp, err := http.Post(base+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("POST /mcp without auth: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /mcp without auth: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	// An allow-listed token reaches the real registry run wires from every
	// use case (VM lifecycle, SSH keys, DNS, SSL, operation status).
	names := listTools(t, base, "smoke-test-token")
	for _, want := range []string{"ping", "create_server", "list_servers", "get_operation_status"} {
		if !names[want] {
			t.Errorf("tools/list is missing %q; got %d tools", want, len(names))
		}
	}

	// Cancel the same way a SIGTERM-derived context would and confirm run()
	// drains the worker pool and returns cleanly instead of hanging or
	// surfacing an error.
	cancel()
	if err := waitDone(t, runErr); err != nil {
		t.Fatalf("run() after shutdown = %v, want nil", err)
	}

	// Startup recovery is exercised in depth by internal/core/app's own
	// tests; the cheapest end-to-end proof here is that a second run against
	// the same DB file — the situation after any real restart — starts
	// cleanly instead of choking while reconciling an already-clean database.
	ctx2, cancel2 := context.WithCancel(context.Background())
	addrCh2 := make(chan net.Addr, 1)
	runErr2 := make(chan error, 1)
	go func() { runErr2 <- run(ctx2, cfg, logger, func(a net.Addr) { addrCh2 <- a }) }()

	waitListening(t, addrCh2)
	cancel2()
	if err := waitDone(t, runErr2); err != nil {
		t.Fatalf("second run() (post-restart) = %v, want nil", err)
	}
}

func waitListening(t *testing.T, addrCh <-chan net.Addr) string {
	t.Helper()
	select {
	case a := <-addrCh:
		return a.String()
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start listening in time")
		return ""
	}
}

func waitDone(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not shut down within the shutdown window")
		return nil
	}
}

func waitFor200(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
		} else {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never returned 200: %v", url, lastErr)
}

func listTools(t *testing.T, base, token string) map[string]bool {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, base+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("building tools/list request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp tools/list: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /mcp tools/list: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding tools/list response: %v", err)
	}

	names := make(map[string]bool, len(out.Result.Tools))
	for _, tool := range out.Result.Tools {
		names[tool.Name] = true
	}
	return names
}
