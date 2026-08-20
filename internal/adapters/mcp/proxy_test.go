package mcp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// remoteServer stands in for a self-hosted do0ps server. It records what the
// proxy sent so the test can assert on the forwarded request, not just the
// answer.
type remoteServer struct {
	*httptest.Server
	requests []rpcRequest
	auth     []string
}

func newRemoteServer(t *testing.T, respond func(rpcRequest) (int, any)) *remoteServer {
	t.Helper()

	remote := &remoteServer{}
	remote.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("remote server got malformed JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		remote.requests = append(remote.requests, req)
		remote.auth = append(remote.auth, r.Header.Get("Authorization"))

		status, body := respond(req)
		if body == nil {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(remote.Close)
	return remote
}

func runProxySession(t *testing.T, endpoint, token, requests string) []map[string]any {
	t.Helper()

	proxy, err := NewProxy(endpoint, token, WithProxyLogger(discardLogger()), WithProxyTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var out strings.Builder
	if err := proxy.ServeStdio(ctx, strings.NewReader(requests), &out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	var got []map[string]any
	dec := json.NewDecoder(strings.NewReader(out.String()))
	for dec.More() {
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			t.Fatalf("decoding response stream %q: %v", out.String(), err)
		}
		got = append(got, msg)
	}
	return got
}

func TestProxyForwardsCallsWithToken(t *testing.T) {
	remote := newRemoteServer(t, func(req rpcRequest) (int, any) {
		return http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": []any{}}}
	})

	got := runProxySession(t, remote.URL, "secret-token",
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1: %v", len(got), got)
	}
	if len(remote.requests) != 1 || remote.requests[0].Method != "tools/list" {
		t.Fatalf("remote saw %v, want one tools/list", remote.requests)
	}
	// The whole point of the bridge: the chat client cannot send this header
	// itself, so the bundle adds it.
	if remote.auth[0] != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want the configured bearer token", remote.auth[0])
	}
}

func TestProxyForwardsNotificationsWithoutReplying(t *testing.T) {
	remote := newRemoteServer(t, func(rpcRequest) (int, any) {
		return http.StatusAccepted, nil
	})

	got := runProxySession(t, remote.URL, "secret-token",
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")

	if len(got) != 0 {
		t.Errorf("got %v, want no reply to a notification", got)
	}
	if len(remote.requests) != 1 {
		t.Errorf("remote saw %d requests, want the notification forwarded", len(remote.requests))
	}
}

func TestProxyReportsRejectedToken(t *testing.T) {
	remote := newRemoteServer(t, func(rpcRequest) (int, any) {
		return http.StatusUnauthorized, nil
	})

	got := runProxySession(t, remote.URL, "wrong-token",
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1: %v", len(got), got)
	}
	rpcErr, ok := got[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("a rejected token produced no error: %v", got[0])
	}
	// The message is what the user reads in their client's log, so it has to
	// say which knob to turn.
	message, _ := rpcErr["message"].(string)
	if !strings.Contains(message, "token") {
		t.Errorf("error message = %q, want it to point at the token", message)
	}
}

func TestProxyReportsUnreachableServer(t *testing.T) {
	remote := newRemoteServer(t, func(rpcRequest) (int, any) { return http.StatusOK, nil })
	endpoint := remote.URL
	remote.Close()

	got := runProxySession(t, endpoint, "secret-token",
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")

	if len(got) != 1 || got[0]["error"] == nil {
		t.Fatalf("an unreachable server produced %v, want an error response", got)
	}
}

func TestNewProxyValidatesConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		token    string
	}{
		{"no scheme", "do0ps.example.com/mcp", "token"},
		{"wrong scheme", "ftp://do0ps.example.com/mcp", "token"},
		{"no host", "https:///mcp", "token"},
		{"no token", "https://do0ps.example.com/mcp", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewProxy(tc.endpoint, tc.token); err == nil {
				t.Errorf("NewProxy(%q, %q) succeeded, want an error", tc.endpoint, tc.token)
			}
		})
	}
}
