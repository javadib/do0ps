// Package arvancloud is the secondary adapter implementing
// ports.ArvanCloudProvider against ArvanCloud's CDN API.
//
// Base URL, auth scheme and error shapes are confirmed against
// docs/api-specs/arvancloud-cdn-4.0.yml ("ArvanCloud CDN Services" 4.180.1),
// which AGENTS.md 4.5's policy for the Parspack specs makes ground truth here
// too. Unlike Parspack, ArvanCloud serves its whole CDN API from a single
// surface, so this client holds one base URL rather than three.
//
// Credentials are never stored: every method receives them from the caller
// (AGENTS.md 4.2).
package arvancloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/javadib/do0ps/internal/core/domain"
)

const (
	defaultBaseURL = "https://napi.arvancloud.ir/cdn/4.0"
	defaultTimeout = 30 * time.Second

	// apiKeyPrefix is the Authorization scheme ArvanCloud API keys use.
	//
	// The spec declares two accepted schemes: UserToken (HTTP Bearer, a JWT
	// from an interactive login) and ApiKey (a raw value in the Authorization
	// header). This adapter uses ApiKey — it is the machine-facing one, and it
	// matches domain.ProviderCredentials.APIKey. The literal scheme word is
	// "Apikey", capital A and nothing else, and it is NOT interchangeable with
	// "Bearer": sending an API key under Bearer is read as a malformed JWT and
	// answered with a 401 that looks like a bad key.
	apiKeyPrefix = "Apikey "

	// defaultUserAgent identifies this client to ArvanCloud. Set explicitly
	// for the same reason as in the Parspack adapter: net/http's default
	// "Go-http-client/1.1" is exactly the User-Agent an edge WAF is most
	// likely to reject, and leaving the header unset is not a neutral choice
	// because net/http fills it in.
	defaultUserAgent = "do0ps/1.0 (+https://github.com/javadib/do0ps)"

	// providerName identifies this adapter's upstream in the errors it
	// returns.
	providerName = "arvancloud"

	// multipartZoneFileField is the request field name the CDN API expects
	// for a BIND zone file upload (dns-records.import, the spec's
	// DnsRecordImport schema: {"f_zone_file": <binary>}).
	multipartZoneFileField = "f_zone_file"

	// defaultMaxRetries is how many times a transient failure is retried
	// before the call gives up, on top of the first attempt. Retry/backoff
	// around the outbound HTTP call is each provider adapter's own concern
	// (AGENTS.md 4.3); the queue's job-level retries sit above this and count
	// a fully exhausted call as one failed attempt.
	defaultMaxRetries = 2
)

// statusTooManyRequests is ArvanCloud's throttling response (the spec's
// TrottleRequests). Treated as transient, like a 5xx.
const statusTooManyRequests = http.StatusTooManyRequests

// Client talks to the ArvanCloud CDN API. It holds no credentials: every
// call receives the caller's own, which belong to the chatbot session
// (AGENTS.md 4.2).
type Client struct {
	baseURL     string
	http        *http.Client
	logger      *slog.Logger
	userAgent   string
	newBackOff  func() backoff.BackOff
	maxRetries  uint64
	nowRetrying func() // test hook, nil in production; see WithRetryHook
}

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL overrides the CDN API root, mainly for tests against a fake
// server.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("base URL must not be empty")
		}
		c.baseURL = strings.TrimSuffix(u, "/")
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

// WithUserAgent overrides the User-Agent sent to ArvanCloud. Callers that
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
// above debug level, and the credential is redacted either way.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) error {
		if l == nil {
			return errors.New("logger must not be nil")
		}
		c.logger = l
		return nil
	}
}

// WithBackOff overrides the backoff sequence used between retries of a
// transient failure. The factory is called once per outbound call, so each
// call retries on a fresh sequence.
func WithBackOff(f func() backoff.BackOff) Option {
	return func(c *Client) error {
		if f == nil {
			return errors.New("backoff factory must not be nil")
		}
		c.newBackOff = f
		return nil
	}
}

// WithMaxRetries overrides how many retries a transient failure gets on top
// of the first attempt. Zero disables retrying.
func WithMaxRetries(n uint64) Option {
	return func(c *Client) error {
		c.maxRetries = n
		return nil
	}
}

// WithRetryHook registers a function called immediately before each retry.
// It exists so tests can assert that a retry actually happened rather than
// inferring it from a request count; production wiring leaves it unset.
func WithRetryHook(f func()) Option {
	return func(c *Client) error {
		if f == nil {
			return errors.New("retry hook must not be nil")
		}
		c.nowRetrying = f
		return nil
	}
}

// defaultBackOff returns exponential backoff with jitter, sized for an
// outbound API call rather than for a background job: the caller of a fast
// operation is a chatbot waiting on a tool result, so the intervals stay
// short. MaxElapsedTime is left unbounded because maxRetries, not wall-clock
// time, is what stops the sequence.
func defaultBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 200 * time.Millisecond
	b.Multiplier = 2
	b.RandomizationFactor = 0.3
	b.MaxInterval = 5 * time.Second
	b.MaxElapsedTime = 0
	return b
}

// New builds an ArvanCloud client.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    defaultBaseURL,
		http:       &http.Client{Timeout: defaultTimeout},
		logger:     slog.New(slog.DiscardHandler),
		userAgent:  defaultUserAgent,
		newBackOff: defaultBackOff,
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("configuring arvancloud client: %w", err)
		}
	}
	return c, nil
}

// envelope is the {"message", "data"} response shape the CDN API wraps its
// payloads in (the spec's DataWithMessageResponse and PaginatedResponse).
// Only "data" is decoded here: "message" is a human-readable confirmation on
// a success path, and the failure paths never reach this struct.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// doJSON sends a request with an optional JSON body and decodes the response
// envelope's "data" into out (nil to discard the body, e.g. for DELETE).
//
// Non-2xx responses are translated into *domain.ProviderError values wrapping
// the sentinels in domain/errors.go, so callers keep classifying failures with
// errors.Is. Transient failures — a request that never reached the provider,
// a 429, or a 5xx — are retried with backoff before the last one is returned.
func (c *Client) doJSON(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}

	var env envelope
	attempt := 0
	operation := func() error {
		if attempt > 0 && c.nowRetrying != nil {
			c.nowRetrying()
		}
		attempt++
		env = envelope{}
		return c.send(ctx, authorization, method, path, encoded, &env)
	}

	sequence := backoff.WithContext(backoff.WithMaxRetries(c.newBackOff(), c.maxRetries), ctx)
	if err := backoff.Retry(operation, sequence); err != nil {
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

// roundTrip performs one attempt of an outbound call and returns the
// response body verbatim, whatever its shape. It is the one place that
// builds the request, applies headers and classifies the response status —
// send, doMultipart and doRawGET all funnel through it, so every request
// shape this client sends (JSON, multipart, and the one raw-response GET)
// retries and maps errors identically.
//
// Failures worth another attempt are returned as-is for the backoff loop;
// everything else is wrapped in backoff.Permanent so the loop stops at once.
// backoff.Retry unwraps a permanent error, so callers still see the
// *domain.ProviderError.
func (c *Client) roundTrip(ctx context.Context, authorization, method, path, contentType, accept string, body []byte) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/"+strings.TrimPrefix(path, "/"), reqBody)
	if err != nil {
		return nil, backoff.Permanent(fmt.Errorf("building %s %s request: %w", method, path, err))
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", accept)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", c.userAgent)

	c.logger.Debug("arvancloud request",
		"method", method, "url", req.URL.String(), "headers", redactedHeaders(req.Header))

	resp, err := c.http.Do(req)
	if err != nil {
		// The request never reached ArvanCloud. Report the reason (timeout,
		// refused connection, DNS) rather than a bare "provider unavailable",
		// and let the backoff loop try again: this is the most common
		// transient failure there is.
		return nil, &domain.ProviderError{
			Provider:  providerName,
			Operation: method + " " + path,
			Message:   transportReason(err),
			Err:       domain.ErrProviderUnavailable,
		}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s %s response: %w", method, path, err)
	}

	c.logger.Debug("arvancloud response",
		"method", method, "path", path, "status", resp.StatusCode, "body", redactedResponseBody(data))

	// Read the body before deciding anything about the status: the body is
	// where the provider says *why* it refused.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		mapped := mapErrorResponse(method, path, resp.StatusCode, data)
		if retryableStatus(resp.StatusCode) {
			return nil, mapped
		}
		return nil, backoff.Permanent(mapped)
	}

	return data, nil
}

// send performs one attempt of an outbound JSON call, decoding a non-empty
// 2xx body into env. See roundTrip for the shared request/retry mechanics.
func (c *Client) send(ctx context.Context, authorization, method, path string, body []byte, env *envelope) error {
	data, err := c.roundTrip(ctx, authorization, method, path, "application/json", "application/json", body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, env); err != nil {
		return backoff.Permanent(fmt.Errorf("decoding %s %s response: %w", method, path, err))
	}
	return nil
}

// doMultipart uploads fileContent as a multipart/form-data request under
// multipartZoneFileField and decodes the response envelope's "data" into out
// (nil to discard), with the same retry/error-mapping behavior as doJSON.
// This is the one request body among ports.ArvanCloudProvider's DNS methods
// that is not JSON (dns-records.import, the spec's DnsRecordImport schema).
func (c *Client) doMultipart(ctx context.Context, creds domain.ProviderCredentials, method, path, fileName string, fileContent []byte, out any) error {
	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}

	var env envelope
	attempt := 0
	operation := func() error {
		if attempt > 0 && c.nowRetrying != nil {
			c.nowRetrying()
		}
		attempt++
		env = envelope{}

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile(multipartZoneFileField, fileName)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("building multipart body for %s %s: %w", method, path, err))
		}
		if _, err := part.Write(fileContent); err != nil {
			return backoff.Permanent(fmt.Errorf("writing multipart body for %s %s: %w", method, path, err))
		}
		if err := writer.Close(); err != nil {
			return backoff.Permanent(fmt.Errorf("closing multipart body for %s %s: %w", method, path, err))
		}

		data, err := c.roundTrip(ctx, authorization, method, path, writer.FormDataContentType(), "application/json", buf.Bytes())
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return backoff.Permanent(fmt.Errorf("decoding %s %s response: %w", method, path, err))
		}
		return nil
	}

	sequence := backoff.WithContext(backoff.WithMaxRetries(c.newBackOff(), c.maxRetries), ctx)
	if err := backoff.Retry(operation, sequence); err != nil {
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

// doCertificateUpload uploads a customer certificate and its private key as
// two multipart/form-data file parts (ssl.cert.store, the CertificateStore
// schema: {"certificate": <binary>, "private_key": <binary>}), decoding the
// response envelope's "data" into out (nil to discard, since ssl.cert.store's
// 201 response carries only a confirmation message — see
// ports.ArvanCloudProvider.UploadArvanCloudCertificate's doc comment), with
// the same retry/error-mapping behavior as doJSON.
//
// privateKey is caller-supplied sensitive material (AGENTS.md 4.2's
// credential-handling principle extended to this field, per issue #73): it
// is sent straight through in the request body, which — like every request
// this client sends — is never logged (see roundTrip's own comment); only
// the method/URL/redacted headers reach the debug log.
func (c *Client) doCertificateUpload(ctx context.Context, creds domain.ProviderCredentials, method, path string, certificate, privateKey []byte, out any) error {
	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}

	var env envelope
	attempt := 0
	operation := func() error {
		if attempt > 0 && c.nowRetrying != nil {
			c.nowRetrying()
		}
		attempt++
		env = envelope{}

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		for _, part := range []struct {
			field, filename string
			content         []byte
		}{
			{"certificate", "certificate.pem", certificate},
			{"private_key", "private_key.pem", privateKey},
		} {
			w, err := writer.CreateFormFile(part.field, part.filename)
			if err != nil {
				return backoff.Permanent(fmt.Errorf("building multipart body for %s %s: %w", method, path, err))
			}
			if _, err := w.Write(part.content); err != nil {
				return backoff.Permanent(fmt.Errorf("writing multipart body for %s %s: %w", method, path, err))
			}
		}
		if err := writer.Close(); err != nil {
			return backoff.Permanent(fmt.Errorf("closing multipart body for %s %s: %w", method, path, err))
		}

		data, err := c.roundTrip(ctx, authorization, method, path, writer.FormDataContentType(), "application/json", buf.Bytes())
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return backoff.Permanent(fmt.Errorf("decoding %s %s response: %w", method, path, err))
		}
		return nil
	}

	sequence := backoff.WithContext(backoff.WithMaxRetries(c.newBackOff(), c.maxRetries), ctx)
	if err := backoff.Retry(operation, sequence); err != nil {
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

// multipartFormFile describes the one optional binary part a doMultipartForm
// request may carry, alongside its plain string fields.
type multipartFormFile struct {
	FieldName string
	FileName  string
	Content   []byte
}

// doMultipartForm sends a multipart/form-data request built from a set of
// plain string fields plus an optional binary file part, and decodes the
// response envelope's "data" into out (nil to discard), with the same
// retry/error-mapping behavior as doJSON. Unlike doMultipart (fixed to the
// single "f_zone_file" field for DNS zone import), this supports the
// several-field forms Custom Pages' two upload endpoints need
// (custom-pages.update: type/page/url/file; custom-pages.file.update:
// active/file) — see edge_settings.go.
func (c *Client) doMultipartForm(
	ctx context.Context, creds domain.ProviderCredentials, method, path string,
	fields map[string]string, file *multipartFormFile, out any,
) error {
	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}

	// Sorted so the request this client sends is deterministic across
	// retries and in tests, even though multipart/form-data does not require
	// field order.
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	var env envelope
	attempt := 0
	operation := func() error {
		if attempt > 0 && c.nowRetrying != nil {
			c.nowRetrying()
		}
		attempt++
		env = envelope{}

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		for _, name := range names {
			if err := writer.WriteField(name, fields[name]); err != nil {
				return backoff.Permanent(fmt.Errorf("building multipart body for %s %s: %w", method, path, err))
			}
		}
		if file != nil {
			part, err := writer.CreateFormFile(file.FieldName, file.FileName)
			if err != nil {
				return backoff.Permanent(fmt.Errorf("building multipart body for %s %s: %w", method, path, err))
			}
			if _, err := part.Write(file.Content); err != nil {
				return backoff.Permanent(fmt.Errorf("writing multipart body for %s %s: %w", method, path, err))
			}
		}
		if err := writer.Close(); err != nil {
			return backoff.Permanent(fmt.Errorf("closing multipart body for %s %s: %w", method, path, err))
		}

		data, err := c.roundTrip(ctx, authorization, method, path, writer.FormDataContentType(), "application/json", buf.Bytes())
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return backoff.Permanent(fmt.Errorf("decoding %s %s response: %w", method, path, err))
		}
		return nil
	}

	sequence := backoff.WithContext(backoff.WithMaxRetries(c.newBackOff(), c.maxRetries), ctx)
	if err := backoff.Retry(operation, sequence); err != nil {
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

// doRawGET performs a GET request whose successful response body is not the
// CDN API's JSON envelope and is returned unparsed — used for
// dns-records.export, whose declared 200 response Content-Type is
// text/plain (a BIND zone file), not application/json. Retries the same
// transient failures as doJSON, via the shared roundTrip.
func (c *Client) doRawGET(ctx context.Context, creds domain.ProviderCredentials, path, accept string) ([]byte, error) {
	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return nil, fmt.Errorf("building GET %s request: %w", path, err)
	}

	var result []byte
	attempt := 0
	operation := func() error {
		if attempt > 0 && c.nowRetrying != nil {
			c.nowRetrying()
		}
		attempt++
		data, err := c.roundTrip(ctx, authorization, http.MethodGet, path, "", accept, nil)
		if err != nil {
			return err
		}
		result = data
		return nil
	}

	sequence := backoff.WithContext(backoff.WithMaxRetries(c.newBackOff(), c.maxRetries), ctx)
	if err := backoff.Retry(operation, sequence); err != nil {
		return nil, err
	}
	return result, nil
}

// doRawJSON sends a request exactly like doJSON, but decodes the response
// body directly into out instead of unwrapping a {"data": ...} envelope
// first — used for account_certificate.install (issue #74/AC14), whose
// response puts success/message as SIBLINGS of data rather than nesting the
// whole payload under "data" the way every other endpoint in this adapter
// does. Retries/error-mapping behave identically to doJSON, via the shared
// roundTrip.
func (c *Client) doRawJSON(ctx context.Context, creds domain.ProviderCredentials, method, path string, body, out any) error {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	authorization, err := authorizationHeader(creds.APIKey)
	if err != nil {
		return fmt.Errorf("building %s %s request: %w", method, path, err)
	}

	var data []byte
	attempt := 0
	operation := func() error {
		if attempt > 0 && c.nowRetrying != nil {
			c.nowRetrying()
		}
		attempt++
		result, err := c.roundTrip(ctx, authorization, method, path, "application/json", "application/json", encoded)
		if err != nil {
			return err
		}
		data = result
		return nil
	}

	sequence := backoff.WithContext(backoff.WithMaxRetries(c.newBackOff(), c.maxRetries), ctx)
	if err := backoff.Retry(operation, sequence); err != nil {
		return err
	}

	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s %s response: %w", method, path, err)
	}
	return nil
}

// retryableStatus reports whether a status is worth another attempt: a
// throttle or a server-side failure may well answer differently in a second.
// A 4xx will not — retrying a rejected credential or an invalid field only
// makes the caller wait longer for the same answer.
func retryableStatus(status int) bool {
	return status == statusTooManyRequests || status >= 500
}

// errorResponse is the failure body the CDN API returns: MessageResponse
// ({"message"}) on most statuses, plus "errors" on a 422.
//
// The 422 "errors" field is deliberately kept raw. The spec allows it to
// arrive in four different shapes (UnprocessableEntityErrorObject, ...Array,
// ...StringArray, ...Arrays) — a field→message object, a field→messages
// object, a flat list of messages, or a list of lists — so it cannot be
// decoded into one Go type up front. This is where ArvanCloud differs from
// Parspack, whose 422 is always {"errors": {"field": ["reason"]}}.
type errorResponse struct {
	Message string          `json:"message"`
	Errors  json.RawMessage `json:"errors"`
}

// unkeyedDetailsField is the key the field-less 422 shapes are filed under,
// so a caller reading ProviderError.Details finds them instead of losing
// them. The two shapes that carry no field names are lists of messages about
// the request as a whole, and "errors" is what the provider itself calls
// them.
const unkeyedDetailsField = "errors"

// mapErrorResponse turns a non-2xx HTTP response into a *domain.ProviderError
// carrying ArvanCloud's own answer — status code, message, per-field
// validation errors, or a raw body excerpt when the payload matches no known
// shape — wrapping the sentinel that classifies it.
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
		provErr.Details = parseValidationErrors(parsed.Errors)
	}

	// Keep the raw body only when nothing structured came out of it — an HTML
	// error page from a proxy in front of the API, or an error shape this
	// adapter does not know, is still worth showing the caller. Redacted the
	// same way as the debug log (redactedResponseBody), in case an
	// unrecognized error shape happens to echo back a sensitive field such as
	// DdosSettings.secret_key.
	if provErr.Message == "" && len(provErr.Details) == 0 {
		provErr.Body = redactedResponseBody(body)
	}
	return provErr
}

// parseValidationErrors normalizes the four shapes a 422's "errors" field can
// take onto the single map[string][]string domain.ProviderError carries. An
// unrecognized shape yields nil rather than a partial map, which leaves
// mapErrorResponse to fall back to the raw body excerpt.
func parseValidationErrors(raw json.RawMessage) map[string][]string {
	if len(raw) == 0 {
		return nil
	}

	// {"field": ["reason", ...]} — UnprocessableEntityErrorArrays.
	var byFieldLists map[string][]string
	if json.Unmarshal(raw, &byFieldLists) == nil && len(byFieldLists) > 0 {
		return byFieldLists
	}

	// {"field": "reason"} — UnprocessableEntityErrorObject.
	var byField map[string]string
	if json.Unmarshal(raw, &byField) == nil && len(byField) > 0 {
		details := make(map[string][]string, len(byField))
		for field, message := range byField {
			details[field] = []string{message}
		}
		return details
	}

	// [["reason", ...], ...] — UnprocessableEntityErrorArray.
	var lists [][]string
	if json.Unmarshal(raw, &lists) == nil && len(lists) > 0 {
		var flat []string
		for _, list := range lists {
			flat = append(flat, list...)
		}
		if len(flat) > 0 {
			return map[string][]string{unkeyedDetailsField: flat}
		}
		return nil
	}

	// ["reason", ...] — UnprocessableEntityErrorStringArray.
	var messages []string
	if json.Unmarshal(raw, &messages) == nil && len(messages) > 0 {
		return map[string][]string{unkeyedDetailsField: messages}
	}
	return nil
}

// sentinelForStatus classifies an HTTP status onto a domain sentinel, or nil
// when the status carries no single meaning.
//
// The mapping follows the responses the spec declares: UnauthorizedError
// (401), NotFoundError (404), UnprocessableEntityError (422), TrottleRequests
// (429) and server-side failures. A 403 (AccessDenied) and a 409 (Conflict)
// get no sentinel: "the key is valid but this account may not do that" and
// "this resource already exists" are both misread if forced onto
// ErrInvalidCredentials or ErrInvalidInput. They still carry the provider's
// full detail, which is exactly what an unclassified failure needs most.
func sentinelForStatus(status int) error {
	switch {
	case status == http.StatusUnauthorized:
		return domain.ErrInvalidCredentials
	case status == http.StatusNotFound:
		return domain.ErrNotFound
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return domain.ErrInvalidInput
	case status == statusTooManyRequests || status >= 500:
		return domain.ErrProviderUnavailable
	default:
		return nil
	}
}

// transportReason describes a failed request attempt without echoing the
// request itself. url.Error stringifies as `Get "<url>": <reason>`, so
// unwrapping to the cause keeps a caller-supplied path segment out of the
// message.
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
// The key arrives as a tool call argument typed or pasted by a human, so it is
// normalized rather than trusted verbatim: surrounding whitespace (a trailing
// newline from a copy-paste is the common one) is trimmed, and an "Apikey "
// prefix the caller already added is stripped so it is not doubled into
// "Apikey Apikey <key>", which ArvanCloud answers with a 401 that reads as if
// the key itself were wrong.
//
// Anything left that cannot go into an HTTP header is rejected here with an
// actionable reason. Without this check net/http fails the request at write
// time with "invalid header field value", which send would report as
// ErrProviderUnavailable — blaming ArvanCloud for a local input problem. The
// offending key is never echoed back: it is a live credential.
func authorizationHeader(apiKey string) (string, error) {
	key := strings.TrimSpace(apiKey)
	for {
		trimmed := trimAPIKeyPrefix(key)
		if trimmed == key {
			break
		}
		key = trimmed
	}
	if key == "" {
		return "", fmt.Errorf("api_key is empty after trimming whitespace and any %q prefix: %w",
			strings.TrimSpace(apiKeyPrefix), domain.ErrInvalidCredentials)
	}

	if !validHeaderValue(key) {
		return "", fmt.Errorf(
			"api_key contains characters that cannot be sent in an HTTP Authorization header "+
				"(such as a newline or a control character from a broken copy-paste); "+
				"pass the key on its own, e.g. \"1a2b3c4d-...\": %w", domain.ErrInvalidCredentials)
	}
	return apiKeyPrefix + key, nil
}

// trimAPIKeyPrefix removes one case-insensitive "Apikey" scheme word, along
// with any whitespace around it, and returns s unchanged when there is none.
// The scheme only counts when the rest of the string starts with whitespace
// or is empty, so a key that merely begins with those letters survives.
func trimAPIKeyPrefix(s string) string {
	scheme := strings.TrimSpace(apiKeyPrefix)
	if len(s) < len(scheme) || !strings.EqualFold(s[:len(scheme)], scheme) {
		return s
	}
	rest := s[len(scheme):]
	if rest != "" && strings.TrimLeft(rest, " \t") == rest {
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
		if b := s[i]; b < 0x20 && b != '\t' || b == 0x7f {
			return false
		}
	}
	return true
}

// redactedHeaders renders the outbound headers for a debug log with the
// credential removed. The Authorization header is reported as its scheme plus
// the key's length — enough to tell "Apikey" apart from a missing prefix or an
// empty key, and never enough to reconstruct the key. A debug log is still a
// log: it gets pasted into issues.
func redactedHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for name, values := range h {
		value := strings.Join(values, ", ")
		if http.CanonicalHeaderKey(name) == "Authorization" {
			scheme, key, found := strings.Cut(value, " ")
			if !found {
				out[name] = fmt.Sprintf("<no scheme, %d chars>", len(value))
				continue
			}
			out[name] = fmt.Sprintf("%s <redacted, %d chars>", scheme, len(key))
			continue
		}
		out[name] = value
	}
	return out
}

// sensitiveResponseFields lists JSON field names that must never appear
// verbatim in the response debug log, wherever they occur in a response
// body — not this adapter's own credential (redactedHeaders above already
// covers that), but material a provider response can still echo back that is
// just as sensitive: DdosSettings.secret_key is the CAPTCHA provider's own
// secret key, caller-supplied and passed straight through (see
// domain.ArvanCloudDdosSettings.SecretKey's doc comment and ddos.go's
// package comment). private_key is CertificateStore's caller-supplied
// certificate private key (see ssl.go's package comment and
// ports.ArvanCloudProvider.UploadArvanCloudCertificate's doc comment) — the
// upload request itself is never logged (roundTrip only logs
// method/URL/redacted headers), but this still guards an unrecognized error
// response that happens to echo the submitted body back, the same
// defense-in-depth mapErrorResponse already relies on for secret_key. A
// debug log is still a log: it gets pasted into issues.
var sensitiveResponseFields = map[string]bool{
	"secret_key":  true,
	"private_key": true,
}

// redactedResponseBody renders a response body for the debug log, replacing
// the value of any sensitiveResponseFields key — at any nesting depth, since
// a settings object can be nested under "data" — with a fixed placeholder
// before truncating. Falls back to the raw truncated body when the response
// is not valid JSON (e.g. an HTML error page): there is nothing structured
// to redact in that case, and TruncateBody's own cap still limits what
// reaches the log.
func redactedResponseBody(body []byte) string {
	var parsed any
	if json.Unmarshal(body, &parsed) != nil {
		return domain.TruncateBody(body)
	}
	redactSensitiveFields(parsed)
	redacted, err := json.Marshal(parsed)
	if err != nil {
		return domain.TruncateBody(body)
	}
	return domain.TruncateBody(redacted)
}

// redactSensitiveFields walks a decoded JSON value in place, blanking any
// object field whose name is in sensitiveResponseFields.
func redactSensitiveFields(v any) {
	switch val := v.(type) {
	case map[string]any:
		for k, sub := range val {
			if s, ok := sub.(string); ok && s != "" && sensitiveResponseFields[k] {
				val[k] = "<redacted>"
				continue
			}
			redactSensitiveFields(sub)
		}
	case []any:
		for _, item := range val {
			redactSensitiveFields(item)
		}
	}
}
