// Package parspack is the secondary adapter implementing
// ports.ParspackProvider against the Parspack API.
//
// The transport concerns — authentication, timeouts, retry/backoff and the
// translation of HTTP failures into domain errors — belong here, and will be
// reintroduced together with the first real endpoint call. Until then every
// provider method marks its work as pending with errNotImplemented so a
// missing piece fails loudly instead of silently returning zero values.
package parspack

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/javadib/do0ps/internal/core/ports"
)

const (
	defaultBaseURL = "https://api.parspack.com"
	defaultTimeout = 30 * time.Second
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
