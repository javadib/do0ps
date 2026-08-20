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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/javadib/do0ps/internal/core/domain"
	"github.com/javadib/do0ps/internal/core/ports"
)

const (
	defaultBaseURL    = "https://my.parspack.com/cserver"
	defaultCDNBaseURL = "https://my.parspack.com/cdnapi"
	defaultSSLBaseURL = "https://my.parspack.com/sslv2"
	defaultTimeout    = 30 * time.Second

	// bearerPrefix is the Authorization scheme all three Parspack surfaces
	// use (AGENTS.md 4.5).
	bearerPrefix = "Bearer "

	// defaultUserAgent identifies this client to Parspack.
	//
	// It must be set explicitly. Parspack's CDN surface sits behind the WAF
	// it sells, and that WAF blocks net/http's default "Go-http-client/1.1"
	// User-Agent outright: the same request that succeeds under any other
	// User-Agent comes back 403 with an "IP Blocked" HTML page under that
	// one. Since net/http adds its default whenever the header is unset, not
	// setting a User-Agent is not a neutral choice here — it is the one value
	// guaranteed to fail.
	defaultUserAgent = "do0ps/1.0 (+https://github.com/javadib/do0ps)"
)

// errEmptyResponse means the provider returned 2xx with no resource in the
// body where one was expected.
var errEmptyResponse = errors.New("parspack returned an empty response body")

// Client talks to the Parspack API. It holds no credentials: every method
// receives the caller's credentials, which belong to the chatbot session
// (AGENTS.md 4.2).
//
// Parspack exposes three distinct API surfaces on the same host under
// different path prefixes (AGENTS.md 4.5). They share nothing but the auth
// scheme, so each gets its own base URL rather than one shared config.
type Client struct {
	baseURL    string // cloud-server surface, e.g. .../cserver
	cdnBaseURL string // CDN surface (zones and DNS records), e.g. .../cdnapi
	sslBaseURL string // SSL ordering surface, e.g. .../sslv2
	http       *http.Client
	logger     *slog.Logger
	userAgent  string
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

// WithUserAgent overrides the User-Agent sent to Parspack. Callers that
// override it must still send something: see defaultUserAgent for why an
// empty one is not an option.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		if strings.TrimSpace(ua) == "" {
			return errors.New("user agent must not be empty")
		}
		c.userAgent = ua
		return nil
	}
}

// WithLogger enables request tracing. Every outbound call is logged at debug
// level before it is sent and again when it answers, which is how a caller
// compares what this adapter sends against a working curl. Nothing is logged
// above debug level, so a default LOG_LEVEL=info deployment traces nothing.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) error {
		if l == nil {
			return errors.New("logger must not be nil")
		}
		c.logger = l
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
		sslBaseURL: defaultSSLBaseURL,
		http:       &http.Client{Timeout: defaultTimeout},
		logger:     slog.New(slog.DiscardHandler),
		userAgent:  defaultUserAgent,
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
//
// The CDN surface adds "errors" to that on 422, mapping each rejected field to
// its reasons (docs/api-specs/parspack-cdn.openapi.yaml), and answers some
// failures inside its {"success","message","data"} envelope rather than with a
// distinct error body — "success" is decoded so a false one is not mistaken
// for an absent message.
type errorResponse struct {
	Message   string              `json:"message"`
	RequestID string              `json:"request_id"`
	Errors    map[string][]string `json:"errors"`
	Success   *bool               `json:"success"`
	Error     string              `json:"error"`
}

// doJSON sends a request against the cloud-server API with an optional JSON
// body and decodes a JSON response into out (nil to discard the body, e.g.
// for DELETE). Non-2xx responses are translated into the sentinel domain
// errors ports callers are expected to check with errors.Is (AGENTS.md 4.2,
// 4.4).
func (c *Client) doJSON(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	return c.doJSONBase(ctx, creds, c.baseURL, method, path, body, out)
}

// doJSONSSL is doJSON against the SSL ordering surface instead of the
// cloud-server one — same auth and error-mapping, different base URL
// (AGENTS.md 4.5).
func (c *Client) doJSONSSL(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	return c.doJSONBase(ctx, creds, c.sslBaseURL, method, path, body, out)
}

// cdnEnvelope is the {"success","message","data"} response shape every CDN
// API endpoint uses (docs/api-specs/parspack-cdn.openapi.yaml), wrapping the payload
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
	if err := c.doJSONBase(ctx, creds, c.cdnBaseURL, method, path, body, &env); err != nil {
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

// doJSONBase is the shared transport doJSON, doJSONSSL and doCDNJSON build
// on, targeting an explicit base URL since Parspack's API surfaces share a
// host but not a path prefix (AGENTS.md 4.5).
func (c *Client) doJSONBase(ctx context.Context, creds domain.ProviderCredentials, baseURL, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+"/"+path, reqBody)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	// Sent on every request, including bodyless GETs. Parspack's own
	// documented calls always carry it, and its CDN surface sits behind the
	// WAF it sells — a request that deviates from the documented shape is
	// exactly what such a filter is built to reject. Matching the documented
	// request costs nothing; diverging from it costs a debugging session.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	c.logger.Debug("parspack request",
		"method", method, "url", req.URL.String(), "headers", redactedHeaders(req.Header))

	resp, err := c.http.Do(req)
	if err != nil {
		// The request never reached Parspack. Report it with the same shape
		// as a rejection so the caller sees the reason (timeout, refused
		// connection, DNS) instead of a bare "provider unavailable".
		return &domain.ProviderError{
			Provider:  providerName,
			Operation: method + " " + path,
			Message:   transportReason(err),
			Err:       domain.ErrProviderUnavailable,
		}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s %s response: %w", method, path, err)
	}

	c.logger.Debug("parspack response",
		"method", method, "path", path, "status", resp.StatusCode, "body", domain.TruncateBody(data))

	// Read the body before deciding anything about the status: the body is
	// where the provider says *why* it refused, and dropping it here is what
	// leaves a caller staring at a bare status code.
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
// complete (docs/api-specs/parspack-cdn.openapi.yaml). Treated the same as a 5xx: it is
// the provider's failure, not a bad request.
const statusFailedDependency = 424

// providerName identifies this adapter's upstream in the errors it returns.
const providerName = "parspack"

// mapErrorResponse turns a non-2xx HTTP response into a *domain.ProviderError
// carrying Parspack's own answer — status code, message, per-field validation
// errors, or a raw body excerpt when the payload matches no known shape — and
// wrapping one of the sentinel domain errors so callers can keep classifying
// it with errors.Is.
//
// The classification follows the status codes AGENTS.md 4.5 confirms for the
// cloud-server API (200/400/401/404/500), plus the 422 and 424 the CDN API
// additionally confirms (docs/api-specs/parspack-cdn.openapi.yaml).
//
// Statuses outside that set (403, 409, ...) carry surface-specific meaning
// that differs between surfaces — e.g. the SSL API's 403 means "order not in
// the right state", not "bad credentials" — so they get no sentinel rather
// than being forced onto one that would mislead callers checking it with
// errors.Is. They still carry the provider's full detail: an unclassified
// failure is exactly the case where the caller most needs to read what
// Parspack actually said.
func mapErrorResponse(method, path string, status int, body []byte) error {
	provErr := &domain.ProviderError{
		Provider:   providerName,
		Operation:  method + " " + path,
		StatusCode: status,
		Err:        sentinelForStatus(status),
	}

	var parsed errorResponse
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil {
		provErr.Message = parsed.Message
		if provErr.Message == "" {
			provErr.Message = parsed.Error
		}
		provErr.Details = parsed.Errors
	}

	// Keep the raw body only when nothing structured came out of it — an
	// HTML error page from a proxy in front of the API, or an error shape
	// this adapter does not know, is still worth showing the caller.
	if provErr.Message == "" && len(provErr.Details) == 0 {
		provErr.Body = domain.TruncateBody(body)
	}
	return provErr
}

// sentinelForStatus classifies an HTTP status onto a domain sentinel, or nil
// when the status has no single meaning across Parspack's three surfaces.
func sentinelForStatus(status int) error {
	switch {
	case status == http.StatusUnauthorized:
		return domain.ErrInvalidCredentials
	case status == http.StatusNotFound:
		return domain.ErrNotFound
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return domain.ErrInvalidInput
	case status == statusFailedDependency || status >= 500:
		return domain.ErrProviderUnavailable
	default:
		return nil
	}
}

// transportReason describes a failed request attempt without echoing the
// request itself. url.Error stringifies as `Get "<url>": <reason>`, and while
// this adapter builds its URLs from path constants, unwrapping to the cause
// keeps a caller-supplied path segment out of the message on principle.
func transportReason(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err.Error()
	}
	return err.Error()
}

// authorizationHeader builds the Authorization header value for an API key
// supplied by the caller.
//
// The key arrives as a tool call argument typed or pasted by a human, so it
// is normalized rather than trusted verbatim: surrounding whitespace (a
// trailing newline from a copy-paste is the common one) is trimmed, and a
// "Bearer " prefix the caller already added is stripped so it is not doubled
// into "Bearer Bearer <token>", which Parspack answers with a 401 that reads
// as if the key itself were wrong.
//
// Anything left that cannot go into an HTTP header is rejected here with an
// actionable reason. Without this check net/http fails the request at write
// time with "invalid header field value", which the transport below would
// wrap as ErrProviderUnavailable — blaming Parspack for a local input
// problem. The offending key is never echoed back: it is a live credential.
func authorizationHeader(apiKey string) (string, error) {
	token := strings.TrimSpace(apiKey)
	for {
		trimmed := trimBearerPrefix(token)
		if trimmed == token {
			break
		}
		token = trimmed
	}
	if token == "" {
		return "", fmt.Errorf("api_key is empty after trimming whitespace and any %q prefix: %w",
			strings.TrimSpace(bearerPrefix), domain.ErrInvalidCredentials)
	}

	if !validHeaderValue(token) {
		return "", fmt.Errorf(
			"api_key contains characters that cannot be sent in an HTTP Authorization header "+
				"(such as a newline or a control character from a broken copy-paste); "+
				"pass the token on its own, e.g. \"9926|abc...\": %w", domain.ErrInvalidCredentials)
	}
	return bearerPrefix + token, nil
}

// trimBearerPrefix removes one case-insensitive "Bearer" scheme word, along
// with any whitespace around it, and returns s unchanged when there is none.
// The scheme only counts when the rest of the string starts with whitespace
// or is empty, so a token that merely begins with those letters survives.
func trimBearerPrefix(s string) string {
	scheme := strings.TrimSpace(bearerPrefix)
	if len(s) < len(scheme) || !strings.EqualFold(s[:len(scheme)], scheme) {
		return s
	}
	rest := s[len(scheme):]
	if rest != "" && strings.TrimLeft(rest, " 	") == rest {
		return s
	}
	return strings.TrimSpace(rest)
}

// validHeaderValue reports whether s may appear in an HTTP header value, per
// RFC 7230 field-content: visible ASCII, space and tab, plus obs-text. It
// mirrors what net/http enforces at write time, so the check here fails the
// call with a useful message instead of an opaque transport error.
func validHeaderValue(s string) bool {
	for i := 0; i < len(s); i++ {
		if b := s[i]; b < 0x20 && b != '	' || b == 0x7f {
			return false
		}
	}
	return true
}

// redactedHeaders renders the outbound headers for a debug log with the
// credential removed. The Authorization header is reported as its scheme plus
// the token's length — enough to tell "Bearer " apart from a missing prefix or
// an empty token, and never enough to reconstruct the key. A debug log is
// still a log: it gets pasted into issues.
func redactedHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for name, values := range h {
		value := strings.Join(values, ", ")
		if http.CanonicalHeaderKey(name) == "Authorization" {
			scheme, token, found := strings.Cut(value, " ")
			if !found {
				out[name] = fmt.Sprintf("<no scheme, %d chars>", len(value))
				continue
			}
			out[name] = fmt.Sprintf("%s <redacted, %d chars>", scheme, len(token))
			continue
		}
		out[name] = value
	}
	return out
}
