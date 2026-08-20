package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultProxyTimeout bounds one forwarded call. Long operations return an
// operation_id immediately, so the slowest thing on this path is a fast
// operation waiting on the provider's own API.
const defaultProxyTimeout = 2 * time.Minute

// Proxy is the other half of the bundle story: instead of running the tools in
// this process, it forwards them over Streamable HTTP to a self-hosted do0ps
// server, adding the configured bearer token.
//
// This is what lets one team run a single server — with its job store, its
// worker pool and its recovery — while everyone installs the same bundle and
// only fills in a URL and a token. The chat client cannot speak HTTP to a
// bearer-guarded endpoint on its own, so the bundled binary bridges the two.
type Proxy struct {
	endpoint string
	token    string
	client   *http.Client
	logger   *slog.Logger
}

// ProxyOption configures a Proxy.
type ProxyOption func(*Proxy) error

// WithProxyLogger sets the logger used for transport failures.
func WithProxyLogger(l *slog.Logger) ProxyOption {
	return func(p *Proxy) error {
		if l == nil {
			return errors.New("logger must not be nil")
		}
		p.logger = l
		return nil
	}
}

// WithProxyTimeout overrides how long one forwarded call may take.
func WithProxyTimeout(d time.Duration) ProxyOption {
	return func(p *Proxy) error {
		if d <= 0 {
			return errors.New("proxy timeout must be positive")
		}
		p.client.Timeout = d
		return nil
	}
}

// NewProxy builds a proxy to the MCP endpoint at endpoint, authenticating with
// token. The endpoint is the full URL of the server's /mcp route.
func NewProxy(endpoint, token string, opts ...ProxyOption) (*Proxy, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("server URL %q is not a valid URL: %w", endpoint, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("server URL %q must start with http:// or https://", endpoint)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("server URL %q has no host", endpoint)
	}
	if token == "" {
		return nil, errors.New("an access token is required when a server URL is configured")
	}

	p := &Proxy{
		endpoint: parsed.String(),
		token:    token,
		client:   &http.Client{Timeout: defaultProxyTimeout},
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, fmt.Errorf("configuring mcp proxy: %w", err)
		}
	}
	return p, nil
}

// Endpoint returns the URL calls are forwarded to. Useful for startup logging.
func (p *Proxy) Endpoint() string { return p.endpoint }

// ServeStdio bridges the chat client's stdio pipe to the remote server. It
// runs the same loop as the in-process server, so framing, concurrency and
// notification handling behave identically on both paths.
func (p *Proxy) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	return serveStdio(ctx, in, out, p.logger, p.dispatch)
}

// dispatch forwards one message and returns the server's answer.
func (p *Proxy) dispatch(ctx context.Context, req rpcRequest) (rpcResponse, bool) {
	isNotification := len(req.ID) == 0

	resp, err := p.forward(ctx, req)
	if err != nil {
		p.logger.Error("forwarding to the do0ps server", "method", req.Method, "error", err)
		if isNotification {
			// Nothing to report it on, and nothing the client can do about it.
			return rpcResponse{}, false
		}
		return errorResponse(req.ID, codeInternalError, err.Error()), true
	}
	if isNotification {
		return rpcResponse{}, false
	}
	return resp, true
}

func (p *Proxy) forward(ctx context.Context, req rpcRequest) (rpcResponse, error) {
	// Re-encode rather than pass the original bytes through: rpcRequest covers
	// every member JSON-RPC 2.0 defines, so nothing meaningful is lost, and the
	// server sees a request this process has actually validated.
	body, err := json.Marshal(req)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("encoding %s request: %w", req.Method, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return rpcResponse{}, fmt.Errorf("building the request to %s: %w", p.endpoint, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("cannot reach the do0ps server at %s: %w", p.endpoint, err)
	}
	defer httpResp.Body.Close()

	// A notification is acknowledged with 202 and no body.
	if httpResp.StatusCode == http.StatusAccepted {
		return rpcResponse{}, nil
	}
	if httpResp.StatusCode != http.StatusOK {
		return rpcResponse{}, statusError(p.endpoint, httpResp)
	}

	var resp rpcResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return rpcResponse{}, fmt.Errorf("the do0ps server at %s returned a malformed response: %w", p.endpoint, err)
	}
	return resp, nil
}

// statusError turns a non-200 into a message that names the likely cause, so
// the user reads it in their chat client's log and knows what to fix.
func statusError(endpoint string, resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("the do0ps server at %s rejected the access token (HTTP %d): check the token in this extension's settings against the server's MCP_AUTH_TOKENS", endpoint, resp.StatusCode)
	case http.StatusNotFound:
		return fmt.Errorf("no MCP endpoint at %s (HTTP 404): the server URL must include the /mcp path", endpoint)
	default:
		// Bounded: an error page should not end up in a log line unabridged.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if trimmed := strings.TrimSpace(string(detail)); trimmed != "" {
			return fmt.Errorf("the do0ps server at %s returned HTTP %d: %s", endpoint, resp.StatusCode, trimmed)
		}
		return fmt.Errorf("the do0ps server at %s returned HTTP %d", endpoint, resp.StatusCode)
	}
}
