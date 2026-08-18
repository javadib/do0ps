// Package parspack is the secondary adapter implementing
// ports.ParspackProvider against the Parspack cloud-server API.
//
// Base URL and auth scheme are confirmed against AGENTS.md 4.5 and
// github.com/abrhacom/go-api-abrha, the Go REST client the Parspack
// cloud-server API is built on (Abrha-based): same host as the CDN and SSL
// surfaces, distinct "/cserver" path prefix, Bearer-token auth. Credentials
// are never stored — every method receives them from the caller.
package parspack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

const (
	defaultBaseURL    = "https://my.parspack.com/cserver"
	defaultSSLBaseURL = "https://my.parspack.com/sslv2"
	defaultTimeout    = 30 * time.Second
)

// errNotImplemented marks provider methods not yet wired to a real endpoint,
// so a missing piece fails loudly instead of silently returning zero values.
var errNotImplemented = errors.New("parspack endpoint not implemented yet")

// errEmptyResponse means the provider returned 2xx with no resource in the
// body where one was expected.
var errEmptyResponse = errors.New("parspack returned an empty response body")

// Client talks to the Parspack cloud-server and SSL APIs — same host and
// Bearer-token auth scheme, distinct path prefixes (AGENTS.md 4.5). It holds
// no credentials: every method receives the caller's credentials, which
// belong to the chatbot session (AGENTS.md 4.2).
type Client struct {
	baseURL    string // cloud-server surface, e.g. .../cserver
	sslBaseURL string // SSL ordering surface, e.g. .../sslv2
	http       *http.Client
}

var _ ports.ParspackProvider = (*Client)(nil)

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL overrides the cloud-server API root, mainly for tests against
// a fake server.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("base URL must not be empty")
		}
		c.baseURL = u
		return nil
	}
}

// WithSSLBaseURL overrides the SSL ordering API root, mainly for tests
// against a fake server.
func WithSSLBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("SSL base URL must not be empty")
		}
		c.sslBaseURL = u
		return nil
	}
}

// WithHTTPClient injects a preconfigured HTTP client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) error {
		if h == nil {
			return errors.New("http client must not be nil")
		}
		c.http = h
		return nil
	}
}

// WithTimeout sets the per-request timeout applied to every outbound call.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if d <= 0 {
			return fmt.Errorf("timeout must be positive, got %s", d)
		}
		c.http.Timeout = d
		return nil
	}
}

// New builds a Parspack client.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    defaultBaseURL,
		sslBaseURL: defaultSSLBaseURL,
		http:       &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("configuring parspack client: %w", err)
		}
	}
	return c, nil
}

// errorResponse is the JSON error body shape used across go-api-abrha-based
// APIs: {"message": "...", "request_id": "..."}.
type errorResponse struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// doJSON sends a request with an optional JSON body to the cloud-server
// surface and decodes a JSON response into out (nil to discard the body,
// e.g. for DELETE). Non-2xx responses are translated into the sentinel
// domain errors ports callers are expected to check with errors.Is
// (AGENTS.md 4.2, 4.4).
func (c *Client) doJSON(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	return c.doJSONBase(ctx, creds, c.baseURL, method, path, body, out)
}

// doJSONSSL is doJSON against the SSL ordering surface instead of the
// cloud-server one — same auth and error-mapping, different base URL
// (AGENTS.md 4.5).
func (c *Client) doJSONSSL(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	return c.doJSONBase(ctx, creds, c.sslBaseURL, method, path, body, out)
}

func (c *Client) doJSONBase(ctx context.Context, creds domain.ProviderCredentials, baseURL, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/"+path, reqBody)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s %s: %w: %v", method, path, domain.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s %s response: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return mapErrorResponse(method, path, resp.StatusCode, data)
	}

	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s %s response: %w", method, path, err)
	}
	return nil
}

// mapErrorResponse turns a non-2xx HTTP response into one of the sentinel
// domain errors, per the status codes AGENTS.md 4.5 confirms across the
// Parspack APIs (200/400/401/404/500). Statuses outside that confirmed set
// (403, 409, 422, ...) carry surface-specific meaning that differs between
// the cloud-server and SSL APIs — e.g. the SSL API's 403 means "order not in
// the right state", not "bad credentials" — so they fall through to a plain
// wrapped error with the provider's message rather than being forced onto a
// sentinel that would mislead callers checking it with errors.Is.
func mapErrorResponse(method, path string, status int, body []byte) error {
	var parsed errorResponse
	message := ""
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil {
		message = parsed.Message
	}
	if message == "" {
		message = fmt.Sprintf("status %d", status)
	}

	switch {
	case status == http.StatusUnauthorized:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrInvalidCredentials)
	case status == http.StatusNotFound:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrNotFound)
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrInvalidInput)
	case status >= 500:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrProviderUnavailable)
	default:
		return fmt.Errorf("%s %s: status %d: %s", method, path, status, message)
	}
}
