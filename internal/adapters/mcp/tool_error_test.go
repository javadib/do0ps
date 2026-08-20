package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"github.com/javadib/do0ps/internal/adapters/providers/parspack"
	"github.com/javadib/do0ps/internal/core/app"
	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

// inlineQueue is ports.Queue reduced to what a fast operation needs: run the
// task on the calling goroutine. The worker pool has its own tests; what
// matters here is that a provider error survives the trip back out.
type inlineQueue struct{}

var _ ports.Queue = inlineQueue{}

func (inlineQueue) Dispatch(ctx context.Context, task ports.Task) (json.RawMessage, error) {
	return task(ctx)
}

func (inlineQueue) Submit(context.Context, *domain.Job) error { return nil }

// testAPIKey is shaped like a real Parspack key. No assertion may ever find
// it in an error the caller receives.
const testAPIKey = "9926|super-secret-token-value"

// newMCPServerAgainstParspack wires the real list_cdn_zones and list_servers
// tools onto a real Parspack client pointed at the given fake provider, and
// exposes them over the real Streamable HTTP transport.
func newMCPServerAgainstParspack(t *testing.T, provider *httptest.Server) *httptest.Server {
	t.Helper()

	client, err := parspack.New(
		parspack.WithBaseURL(provider.URL),
		parspack.WithCDNBaseURL(provider.URL),
	)
	if err != nil {
		t.Fatalf("parspack.New() error = %v", err)
	}

	queue := inlineQueue{}
	s, err := NewServer([]Tool{
		listCDNZonesTool(app.NewListCDNZones(queue, client)),
		listServersTool(app.NewListServers(queue, client)),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	fiberApp := fiber.New()
	s.Register(fiberApp)

	srv := httptest.NewServer(adaptor.FiberApp(fiberApp))
	t.Cleanup(srv.Close)
	return srv
}

// toolCallResult is the MCP tools/call result shape.
type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// TestToolCallSurfacesParspackError is the regression this whole path exists
// for: a tool whose provider call fails must hand the caller Parspack's own
// answer — status code and message — not a generic "tool failed".
func TestToolCallSurfacesParspackError(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		tool         string
		wantContains []string
	}{
		{
			name:         "403 with a JSON error body",
			status:       http.StatusForbidden,
			body:         `{"message":"IP not whitelisted"}`,
			tool:         "list_cdn_zones",
			wantContains: []string{"403", "IP not whitelisted"},
		},
		{
			name:         "401 on the cloud-server surface",
			status:       http.StatusUnauthorized,
			body:         `{"message":"Unauthenticated."}`,
			tool:         "list_servers",
			wantContains: []string{"401", "Unauthenticated."},
		},
		{
			name:   "422 reports the rejected field",
			status: http.StatusUnprocessableEntity,
			body:   `{"message":"The given data was invalid.","errors":{"domain":["The domain is required."]}}`,
			tool:   "list_cdn_zones",
			wantContains: []string{
				"422", "The given data was invalid.", "domain", "The domain is required.",
			},
		},
		{
			name:         "500 blames the provider but says what it said",
			status:       http.StatusInternalServerError,
			body:         `{"message":"Internal error"}`,
			tool:         "list_servers",
			wantContains: []string{"500", "Internal error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer provider.Close()

			srv := newMCPServerAgainstParspack(t, provider)
			out := doRPC(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+
				tc.tool+`","arguments":{"api_key":"`+testAPIKey+`"}}}`)

			// A failed provider call is a tool execution error, which MCP
			// reports in the result so the model can read it — not as a
			// JSON-RPC protocol error, which clients treat as a broken tool.
			if out.Error != nil {
				t.Fatalf("got a JSON-RPC protocol error %+v, want a tool result with isError", out.Error)
			}

			var result toolCallResult
			if err := json.Unmarshal(out.Result, &result); err != nil {
				t.Fatalf("decoding tools/call result: %v", err)
			}
			if !result.IsError {
				t.Errorf("isError = false, want true for a failed provider call")
			}
			if len(result.Content) == 0 {
				t.Fatal("result carried no content")
			}

			text := result.Content[0].Text
			for _, want := range tc.wantContains {
				if !strings.Contains(text, want) {
					t.Errorf("tool result = %q, want it to mention %q", text, want)
				}
			}
			if strings.Contains(strings.ToLower(text), "tool "+tc.tool+" failed") {
				t.Errorf("tool result = %q, want the provider's detail, not a generic failure", text)
			}
			if strings.Contains(text, testAPIKey) || strings.Contains(text, "super-secret-token-value") {
				t.Errorf("tool result = %q, must never echo the credential", text)
			}
		})
	}
}

// TestToolCallSucceedsWithoutIsError guards the other half: the error path
// must not have made successful calls look like failures.
func TestToolCallSucceedsWithoutIsError(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"vms":[{"id":"1","name":"web","status":"active"}]}`))
	}))
	defer provider.Close()

	srv := newMCPServerAgainstParspack(t, provider)
	out := doRPC(t, srv.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_servers",`+
		`"arguments":{"api_key":"`+testAPIKey+`"}}}`)

	if out.Error != nil {
		t.Fatalf("tools/call error: %+v", out.Error)
	}

	var result toolCallResult
	if err := json.Unmarshal(out.Result, &result); err != nil {
		t.Fatalf("decoding tools/call result: %v", err)
	}
	if result.IsError {
		t.Errorf("isError = true for a successful call: %+v", result.Content)
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, `"web"`) {
		t.Errorf("content = %+v, want the server list", result.Content)
	}
}

// TestMalformedArgumentsStayProtocolErrors keeps the split honest: a bad
// argument never reached a tool, so it is still a JSON-RPC error, and it still
// says what was wrong with it.
func TestMalformedArgumentsStayProtocolErrors(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("provider was called despite malformed arguments")
	}))
	defer provider.Close()

	srv := newMCPServerAgainstParspack(t, provider)
	out := doRPC(t, srv.URL,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_servers","arguments":{"api_key":123}}}`)

	if out.Error == nil {
		t.Fatalf("expected a JSON-RPC error for malformed arguments, got result %s", out.Result)
	}
	if out.Error.Code != codeInvalidParams {
		t.Errorf("error code = %d, want %d", out.Error.Code, codeInvalidParams)
	}
	if !strings.Contains(out.Error.Message, "api_key") {
		t.Errorf("error message = %q, want it to name the offending argument", out.Error.Message)
	}
}
