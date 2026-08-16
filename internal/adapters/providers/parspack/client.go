// Package parspack is the secondary adapter implementing
// ports.ParspackProvider against the Parspack API.
//
// The transport concerns — authentication, timeouts, retry/backoff and the
// translation of HTTP failures into domain errors — are implemented here.
// The endpoint paths and response shapes still have to be filled in from the
// Parspack API documentation; every method below marks that with
// errNotImplemented so a missing piece fails loudly instead of silently
// returning zero values.
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
	defaultBaseURL = "https://api.parspack.com"
	defaultTimeout = 30 * time.Second
	maxAttempts    = 4
	baseBackoff    = 500 * time.Millisecond
	maxBodyBytes   = 4 << 20 // 4 MiB: refuse to buffer an unbounded response
)

var errNotImplemented = errors.New("parspack endpoint not implemented yet")

// Client talks to the Parspack API. It holds no credentials: every method
// receives the caller's credentials, which belong to the chatbot session.
type Client struct {
	baseURL string
	http    *http.Client
}

var _ ports.ParspackProvider = (*Client)(nil)

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL overrides the API root, mainly for tests against a fake server.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("base URL must not be empty")
		}
		c.baseURL = u
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
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("configuring parspack client: %w", err)
		}
	}
	return c, nil
}

// do performs a request with retry/backoff and decodes the JSON response into
// out. Retries stop on context cancellation and on non-retryable statuses.
func (c *Client) do(
	ctx context.Context,
	creds domain.ProviderCredentials,
	method, path string,
	body, out any,
) error {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		payload = encoded
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("calling parspack %s %s: %w", method, path, err)
		}

		err := c.attempt(ctx, creds, method, path, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err

		if !retryable(err) || attempt == maxAttempts {
			return fmt.Errorf("calling parspack %s %s: %w", method, path, err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("calling parspack %s %s: %w", method, path, ctx.Err())
		case <-time.After(backoffFor(attempt)):
		}
	}
	return fmt.Errorf("calling parspack %s %s: %w", method, path, lastErr)
}

func (c *Client) attempt(
	ctx context.Context,
	creds domain.ProviderCredentials,
	method, path string,
	payload []byte,
	out any,
) error {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s", domain.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return statusError(resp)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// statusError maps an HTTP status onto a domain sentinel so use cases and the
// MCP adapter never inspect status codes.
func statusError(resp *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w (http %d)", domain.ErrInvalidCredentials, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w (http %d)", domain.ErrNotFound, resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return fmt.Errorf("%w (http %d): %s", domain.ErrProviderUnavailable, resp.StatusCode, snippet)
	default:
		return fmt.Errorf("%w (http %d): %s", domain.ErrInvalidInput, resp.StatusCode, snippet)
	}
}

// retryable reports whether another attempt could plausibly succeed.
func retryable(err error) bool {
	return errors.Is(err, domain.ErrProviderUnavailable)
}

func backoffFor(attempt int) time.Duration {
	return baseBackoff * time.Duration(1<<(attempt-1))
}
