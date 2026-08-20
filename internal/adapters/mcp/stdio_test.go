package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
)

// echoTool is a stand-in for a real provider tool: it only proves the message
// reached a handler and the result came back through the framing.
func echoTool() Tool {
	return Tool{
		Name:        "echo",
		Description: "Return the value it was given.",
		InputSchema: map[string]any{"type": "object"},
		Handler: func(_ context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Value string `json:"value"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, domain.ErrInvalidInput
			}
			return map[string]any{"value": in.Value}, nil
		},
	}
}

// runStdioSession runs the loop over a canned request stream and returns every
// response it wrote, keyed by request id.
//
// Keyed rather than ordered on purpose: the loop answers requests
// concurrently, so a slow tool call does not hold up a ping queued behind it,
// and JSON-RPC clients match responses by id rather than by position.
func runStdioSession(t *testing.T, requests string) map[float64]map[string]any {
	t.Helper()

	srv, err := NewServer([]Tool{echoTool()}, WithInfo(Info{Name: "do0ps", Version: "1.2.3"}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var out strings.Builder
	if err := srv.ServeStdio(ctx, strings.NewReader(requests), &out); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}

	got := make(map[float64]map[string]any)
	dec := json.NewDecoder(strings.NewReader(out.String()))
	for {
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				return got
			}
			t.Fatalf("decoding response stream %q: %v", out.String(), err)
		}

		id, ok := msg["id"].(float64)
		if !ok {
			t.Fatalf("response carries no id: %v", msg)
		}
		got[id] = msg
	}
}

func TestServeStdioHandshake(t *testing.T) {
	got := runStdioSession(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`+"\n")

	if len(got) != 1 {
		t.Fatalf("got %d responses, want 1: %v", len(got), got)
	}

	result, ok := got[1]["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize returned no result: %v", got[0])
	}
	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("protocolVersion = %v, want %v", result["protocolVersion"], ProtocolVersion)
	}

	info, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("initialize returned no serverInfo: %v", result)
	}
	// The version travels from the build stamp to the client through here, so
	// an installed bundle reports the version it was packed as.
	if info["name"] != "do0ps" || info["version"] != "1.2.3" {
		t.Errorf("serverInfo = %v, want name do0ps version 1.2.3", info)
	}
}

func TestServeStdioListsAndCallsTools(t *testing.T) {
	got := runStdioSession(t, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"value":"hi"}}}`,
		"",
	}, "\n"))

	if len(got) != 2 {
		t.Fatalf("got %d responses, want 2: %v", len(got), got)
	}

	listed := got[1]["result"].(map[string]any)["tools"].([]any)
	if len(listed) != 1 || listed[0].(map[string]any)["name"] != "echo" {
		t.Errorf("tools/list = %v, want the echo tool", listed)
	}

	content := got[2]["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if text != `{"value":"hi"}` {
		t.Errorf("tools/call text = %s, want {\"value\":\"hi\"}", text)
	}
}

func TestServeStdioIgnoresNotifications(t *testing.T) {
	// notifications/initialized carries no id; answering it would desynchronize
	// clients that match responses to requests by position.
	got := runStdioSession(t, strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		"",
	}, "\n"))

	if len(got) != 1 {
		t.Fatalf("got %d responses, want only the ping reply: %v", len(got), got)
	}
	if _, ok := got[1]; !ok {
		t.Errorf("responses = %v, want only the reply to id 1", got)
	}
}

func TestServeStdioRejectsUnknownTool(t *testing.T) {
	got := runStdioSession(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`+"\n")

	rpcErr, ok := got[1]["error"].(map[string]any)
	if !ok {
		t.Fatalf("unknown tool returned no error: %v", got[1])
	}
	if rpcErr["code"] != float64(codeInvalidParams) {
		t.Errorf("error code = %v, want %d", rpcErr["code"], codeInvalidParams)
	}
}

func TestServeStdioStopsOnMalformedInput(t *testing.T) {
	srv, err := NewServer([]Tool{echoTool()})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	var out strings.Builder
	err = srv.ServeStdio(context.Background(), strings.NewReader("{not json\n"), &out)
	if err == nil {
		t.Fatal("ServeStdio accepted a malformed stream, want an error")
	}
}
