package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

// newTestHTTPServer wires a Server behind a real fiber.App exposed over a
// real net/http listener (via the adaptor middleware), so tests exercise the
// same POST+GET Streamable HTTP transport a real MCP client would use rather
// than calling handlers directly.
func newTestHTTPServer(t *testing.T, opts ...Option) *httptest.Server {
	t.Helper()

	s, err := NewServer([]Tool{PingTool()}, opts...)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	app := fiber.New()
	s.Register(app)

	srv := httptest.NewServer(adaptor.FiberApp(app))
	t.Cleanup(srv.Close)
	return srv
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

func doRPC(t *testing.T, baseURL, body string) rpcEnvelope {
	t.Helper()

	resp, err := http.Post(baseURL+"/mcp", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	var out rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding JSON-RPC response: %v", err)
	}
	return out
}

// TestToolsListAndCallPing walks the full registry round trip an MCP client
// relies on: discover the ping tool via tools/list, then call it and read
// the result back through the same JSON-RPC envelope.
func TestToolsListAndCallPing(t *testing.T) {
	srv := newTestHTTPServer(t)

	listOut := doRPC(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if listOut.Error != nil {
		t.Fatalf("tools/list error: %+v", listOut.Error)
	}
	var list struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(listOut.Result, &list); err != nil {
		t.Fatalf("decoding tools/list result: %v", err)
	}
	if len(list.Tools) != 1 || list.Tools[0].Name != "ping" {
		t.Fatalf("tools = %+v, want a single %q tool", list.Tools, "ping")
	}

	callOut := doRPC(t, srv.URL, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}`)
	if callOut.Error != nil {
		t.Fatalf("tools/call error: %+v", callOut.Error)
	}
	var call struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(callOut.Result, &call); err != nil {
		t.Fatalf("decoding tools/call result: %v", err)
	}
	if len(call.Content) != 1 || !strings.Contains(call.Content[0].Text, "pong") {
		t.Fatalf("content = %+v, want a %q reply", call.Content, "pong")
	}
}

func TestHandleRPCUnknownToolIsInvalidParams(t *testing.T) {
	srv := newTestHTTPServer(t)

	out := doRPC(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"does-not-exist","arguments":{}}}`)
	if out.Error == nil {
		t.Fatal("expected an error for an unknown tool, got none")
	}
	if out.Error.Code != codeInvalidParams {
		t.Fatalf("error code = %d, want %d", out.Error.Code, codeInvalidParams)
	}
}

func TestHandleRPCMalformedJSONIsInvalidRequest(t *testing.T) {
	srv := newTestHTTPServer(t)

	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var out rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding JSON-RPC response: %v", err)
	}
	if out.Error == nil || out.Error.Code != codeInvalidRequest {
		t.Fatalf("error = %+v, want code %d", out.Error, codeInvalidRequest)
	}
}

// TestSSEStreamRoundTrip proves the streaming half of Streamable HTTP end to
// end (AGENTS.md 5): a real client connects over GET, receives the initial
// "ready" event, and disconnecting is detected server-side (via a short
// heartbeat so the test doesn't wait out the middleware's 15s default)
// instead of leaking the handler goroutine.
func TestSSEStreamRoundTrip(t *testing.T) {
	srv := newTestHTTPServer(t, WithHeartbeatInterval(20*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp", http.NoBody)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get(fiber.HeaderContentType); ct != fiber.MIMETextEventStream {
		t.Fatalf("Content-Type = %q, want %q", ct, fiber.MIMETextEventStream)
	}

	reader := bufio.NewReader(resp.Body)
	var readyLine string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading SSE stream: %v", err)
		}
		if strings.HasPrefix(line, "event:") {
			readyLine = line
			break
		}
	}
	if !strings.Contains(readyLine, "ready") {
		t.Fatalf("first event line = %q, want the ready event", readyLine)
	}

	// Disconnecting here must not hang the server: httptest.Server.Close (via
	// t.Cleanup) blocks until every handler returns, so a leaked stream
	// goroutine would hang the whole test instead of just failing it.
}
