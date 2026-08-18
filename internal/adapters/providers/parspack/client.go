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
	defaultCDNBaseURL = "https://my.parspack.com/cdnapi"
	defaultTimeout    = 30 * time.Second
)

// errNotImplemented marks provider methods not yet wired to a real endpoint,
// so a missing piece fails loudly instead of silently returning zero values.
var errNotImplemented = errors.New("parspack endpoint not implemented yet")

// errEmptyResponse means the provider returned 2xx with no resource in the
// body where one was expected.
var errEmptyResponse = errors.New("parspack returned an empty response body")

// Client talks to the Parspack API. It holds no credentials: every method
// receives the caller's credentials, which belong to the chatbot session
// (AGENTS.md 4.2).
//
// Parspack exposes distinct API surfaces on the same host but different path
// prefixes (AGENTS.md 4.5): baseURL is the cloud-server surface ("/cserver",
// VM lifecycle, SSH keys), cdnBaseURL is the CDN surface ("/cdnapi", CDN zone
// management and DNS records, issue #19). They share nothing but the auth
// scheme, so each gets its own base URL rather than one shared config.
type Client struct {
	baseURL    string
	cdnBaseURL string
	http       *http.Client
}

var _ ports.ParspackProvider = (*Client)(nil)

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL overrides the cloud-server API root, mainly for tests against a
// fake server.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("base URL must not be empty")
		}
		c.baseURL = u
		return nil
	}
}

// WithCDNBaseURL overrides the CDN API root, mainly for tests against a fake
// server.
func WithCDNBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("CDN base URL must not be empty")
		}
		c.cdnBaseURL = u
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
		cdnBaseURL: defaultCDNBaseURL,
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

// doJSON sends a request against the cloud-server API with an optional JSON
// body and decodes a JSON response into out (nil to discard the body, e.g.
// for DELETE). Non-2xx responses are translated into the sentinel domain
// errors ports callers are expected to check with errors.Is (AGENTS.md 4.2,
// 4.4).
func (c *Client) doJSON(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	return c.doJSONAt(ctx, creds, c.baseURL, method, path, body, out)
}

// cdnEnvelope is the {"success","message","data"} response shape every CDN
// API endpoint uses (docs/api-specs/cdn-api.openapi), wrapping the payload
// doJSON's cloud-server callers get directly.
type cdnEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// doCDNJSON is doJSON's counterpart for the CDN API surface (AGENTS.md 4.5):
// same auth scheme and error handling, different base URL and response
// envelope. It unwraps the envelope's "data" field into out.
func (c *Client) doCDNJSON(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	var env cdnEnvelope
	if err := c.doJSONAt(ctx, creds, c.cdnBaseURL, method, path, body, &env); err != nil {
		return err
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decoding %s %s response data: %w", method, path, err)
	}
	return nil
}

// doJSONAt is the shared transport doJSON and doCDNJSON build on, targeting
// an explicit base URL since Parspack's API surfaces share a host but not a
// path prefix (AGENTS.md 4.5).
func (c *Client) doJSONAt(ctx context.Context, creds domain.ProviderCredentials, baseURL, method, path string, body, out any) error {
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

// statusFailedDependency is the CDN API's "Operation Fail" status (424),
// used when a request was valid but the underlying operation could not
// complete (docs/api-specs/cdn-api.openapi). Treated the same as a 5xx: it is
// the provider's failure, not a bad request.
const statusFailedDependency = 424

// mapErrorResponse turns a non-2xx HTTP response into one of the sentinel
// domain errors, per the status codes AGENTS.md 4.5 confirms for the
// cloud-server API (200/400/401/404/500) and the CDN API additionally
// confirms (403/422/424, docs/api-specs/cdn-api.openapi).
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
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrInvalidCredentials)
	case status == http.StatusNotFound:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrNotFound)
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrInvalidInput)
	case status == statusFailedDependency || status >= 500:
		return fmt.Errorf("%s %s: %s: %w", method, path, message, domain.ErrProviderUnavailable)
	default:
		return fmt.Errorf("%s %s: unexpected status %d: %s", method, path, status, message)
	}
}
