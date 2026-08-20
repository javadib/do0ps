package arvancloud

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/javadib/do0ps/internal/core/domain"
)

// realKey is shaped like an ArvanCloud API key: a UUID.
const realKey = "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"

// creds is the credential every test call passes; the client holds none of
// its own (AGENTS.md 4.2).
func creds() domain.ProviderCredentials {
	return domain.ProviderCredentials{APIKey: realKey}
}

// newTestClient builds a client against srv with retrying made instant, so a
// test that exercises the backoff path does not spend real seconds in it.
func newTestClient(t *testing.T, srv *httptest.Server, opts ...Option) *Client {
	t.Helper()
	base := []Option{
		WithBaseURL(srv.URL),
		WithBackOff(func() backoff.BackOff { return &backoff.ZeroBackOff{} }),
	}
	client, err := New(append(base, opts...)...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

// TestAuthorizationHeaderScheme pins the one detail an ArvanCloud key is most
// often gotten wrong on: the scheme word is "Apikey", not "Bearer". The spec
// declares both schemes, and sending an API key under the JWT one comes back
// as a 401 that reads like a bad key.
func TestAuthorizationHeaderScheme(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":{"name":"example.com"}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	if err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains", nil, nil); err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}

	if want := "Apikey " + realKey; seen != want {
		t.Errorf("Authorization header = %q, want %q", seen, want)
	}
}

// TestAuthorizationHeaderNormalization covers the shapes an api_key tool
// argument actually arrives in. A chatbot passes the raw key, but a user
// pasting from the ArvanCloud panel or from a curl command may include the
// scheme word or a trailing newline; all of those must reach the provider as
// one well-formed header.
func TestAuthorizationHeaderNormalization(t *testing.T) {
	want := "Apikey " + realKey

	valid := []struct {
		name   string
		apiKey string
	}{
		{"raw key", realKey},
		{"apikey prefixed", "Apikey " + realKey},
		{"lowercase apikey prefix", "apikey " + realKey},
		{"surrounding whitespace", "  " + realKey + "  "},
		{"trailing newline from a copy-paste", realKey + "\n"},
		{"trailing CRLF from a copy-paste", realKey + "\r\n"},
		{"prefix doubled by a caller that also prepends", "Apikey Apikey " + realKey},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := authorizationHeader(tc.apiKey)
			if err != nil {
				t.Fatalf("authorizationHeader() error = %v, want nil", err)
			}
			if got != want {
				t.Errorf("authorizationHeader() = %q, want %q", got, want)
			}
		})
	}

	invalid := []struct {
		name   string
		apiKey string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"apikey prefix with no key", "Apikey "},
		{"embedded newline", "1a2b\n3c4d"},
		{"embedded carriage return", "1a2b\r3c4d"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := authorizationHeader(tc.apiKey)
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("authorizationHeader() = %q, error = %v, want ErrInvalidCredentials", got, err)
			}
			if tc.apiKey != "" && strings.Contains(err.Error(), tc.apiKey) {
				t.Errorf("error message %q echoes the credential back", err)
			}
		})
	}
}

// TestStatusMapping checks that each status the CDN spec declares reaches the
// caller as the sentinel it is supposed to classify as, with the provider's
// own message preserved.
func TestStatusMapping(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantErr     error
		wantMessage string
	}{
		{
			name:        "401 unauthorized",
			status:      http.StatusUnauthorized,
			body:        `{"message":"Unauthenticated."}`,
			wantErr:     domain.ErrInvalidCredentials,
			wantMessage: "Unauthenticated.",
		},
		{
			name:        "404 not found",
			status:      http.StatusNotFound,
			body:        `{"status":false,"message":"Domain not found."}`,
			wantErr:     domain.ErrNotFound,
			wantMessage: "Domain not found.",
		},
		{
			name:        "400 bad request",
			status:      http.StatusBadRequest,
			body:        `{"message":"Bad request."}`,
			wantErr:     domain.ErrInvalidInput,
			wantMessage: "Bad request.",
		},
		{
			name:        "500 server error",
			status:      http.StatusInternalServerError,
			body:        `{"message":"Server Error."}`,
			wantErr:     domain.ErrProviderUnavailable,
			wantMessage: "Server Error.",
		},
		{
			name:        "429 throttled",
			status:      http.StatusTooManyRequests,
			body:        `{"message":"Too Many Attempts."}`,
			wantErr:     domain.ErrProviderUnavailable,
			wantMessage: "Too Many Attempts.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			client := newTestClient(t, srv, WithMaxRetries(0))
			err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains/example.com", nil, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("doJSON() error = %v, want %v", err, tc.wantErr)
			}

			var provErr *domain.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("doJSON() error = %v, want a *domain.ProviderError", err)
			}
			if provErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", provErr.StatusCode, tc.status)
			}
			if provErr.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", provErr.Message, tc.wantMessage)
			}
			if provErr.Provider != providerName {
				t.Errorf("Provider = %q, want %q", provErr.Provider, providerName)
			}
		})
	}
}

// TestUnclassifiedStatusKeepsDetail covers the statuses deliberately left
// without a sentinel: a 403 means "this account may not do that", not "bad
// key", so it must not be forced onto ErrInvalidCredentials — while still
// carrying what the provider said.
func TestUnclassifiedStatusKeepsDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Access denied."}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, WithMaxRetries(0))
	err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains", nil, nil)

	var provErr *domain.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("doJSON() error = %v, want a *domain.ProviderError", err)
	}
	for _, sentinel := range []error{
		domain.ErrInvalidCredentials, domain.ErrNotFound,
		domain.ErrInvalidInput, domain.ErrProviderUnavailable,
	} {
		if errors.Is(err, sentinel) {
			t.Errorf("403 classified as %v; it has no single meaning and must carry no sentinel", sentinel)
		}
	}
	if provErr.Message != "Access denied." {
		t.Errorf("Message = %q, want %q", provErr.Message, "Access denied.")
	}
}

// TestValidationErrorShapes pins the 422 handling, the place ArvanCloud
// differs most from Parspack: the spec allows "errors" to arrive in four
// different shapes, and all four must reach the caller as ProviderError
// Details rather than being dropped.
func TestValidationErrorShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want map[string][]string
	}{
		{
			name: "field to list of messages",
			body: `{"status":false,"message":"The given data was invalid.",` +
				`"errors":{"domain":["The domain field is required.","Invalid hostname."]}}`,
			want: map[string][]string{"domain": {"The domain field is required.", "Invalid hostname."}},
		},
		{
			name: "field to single message",
			body: `{"status":false,"message":"The given data was invalid.",` +
				`"errors":{"plan_level":"Selected plan is not available."}}`,
			want: map[string][]string{"plan_level": {"Selected plan is not available."}},
		},
		{
			name: "flat list of messages",
			body: `{"status":false,"message":"The given data was invalid.",` +
				`"errors":["The domain field is required."]}`,
			want: map[string][]string{"errors": {"The domain field is required."}},
		},
		{
			name: "list of lists of messages",
			body: `{"status":false,"message":"The given data was invalid.",` +
				`"errors":[["The domain field is required."],["Invalid hostname."]]}`,
			want: map[string][]string{"errors": {"The domain field is required.", "Invalid hostname."}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			client := newTestClient(t, srv, WithMaxRetries(0))
			err := client.doJSON(context.Background(), creds(), http.MethodPost, "domains/dns-service", map[string]string{"domain": ""}, nil)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("doJSON() error = %v, want ErrInvalidInput", err)
			}

			var provErr *domain.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("doJSON() error = %v, want a *domain.ProviderError", err)
			}
			if provErr.Message != "The given data was invalid." {
				t.Errorf("Message = %q, want %q", provErr.Message, "The given data was invalid.")
			}
			if !equalDetails(provErr.Details, tc.want) {
				t.Errorf("Details = %v, want %v", provErr.Details, tc.want)
			}
		})
	}
}

// TestUnknownErrorBodyKeepsExcerpt covers a failure that matches no known
// shape — an HTML page from a proxy in front of the API is the usual one.
// Reducing that to a bare status code is what leaves a caller with nothing to
// go on.
func TestUnknownErrorBodyKeepsExcerpt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html><body>404 Not Found</body></html>"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, WithMaxRetries(0))
	err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains/example.com", nil, nil)

	var provErr *domain.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("doJSON() error = %v, want a *domain.ProviderError", err)
	}
	if !strings.Contains(provErr.Body, "404 Not Found") {
		t.Errorf("Body = %q, want it to carry the raw response excerpt", provErr.Body)
	}
}

// TestTransportFailureMapsToUnavailable covers a request that never reaches
// ArvanCloud: the server is closed before the call, so the connection is
// refused.
func TestTransportFailureMapsToUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	client, err := New(
		WithBaseURL(url),
		WithBackOff(func() backoff.BackOff { return &backoff.ZeroBackOff{} }),
		WithMaxRetries(0),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = client.doJSON(context.Background(), creds(), http.MethodGet, "domains", nil, nil)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("doJSON() error = %v, want ErrProviderUnavailable", err)
	}

	var provErr *domain.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("doJSON() error = %v, want a *domain.ProviderError", err)
	}
	if provErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 for a request that never got a response", provErr.StatusCode)
	}
	if provErr.Message == "" {
		t.Error("Message is empty; the transport reason is what tells the caller why the call failed")
	}
}

// TestRetriesTransientFailure proves the backoff wrapper actually retries: one
// 503, then a success, and the caller sees only the success.
func TestRetriesTransientFailure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"Service Unavailable."}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"name":"example.com","status":"active"}}`))
	}))
	defer srv.Close()

	var retries atomic.Int32
	client := newTestClient(t, srv, WithRetryHook(func() { retries.Add(1) }))

	var out struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains/example.com", nil, &out); err != nil {
		t.Fatalf("doJSON() error = %v, want the retry to succeed", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("upstream calls = %d, want 2 (one transient failure, one success)", got)
	}
	if got := retries.Load(); got != 1 {
		t.Errorf("retries = %d, want 1", got)
	}
	if out.Name != "example.com" || out.Status != "active" {
		t.Errorf("decoded payload = %+v, want the second response's data", out)
	}
}

// TestDoesNotRetryClientErrors is the other half of the retry contract: a
// rejected credential or an invalid field answers the same way every time, so
// retrying only makes the caller wait longer for it.
func TestDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthenticated."}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, WithMaxRetries(3))
	err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains", nil, nil)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("doJSON() error = %v, want ErrInvalidCredentials", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1: a 401 must not be retried", got)
	}
}

// TestRetriesAreBounded checks that a provider that is down throughout does
// not retry forever: the caller gets the last failure after maxRetries+1
// attempts.
func TestRetriesAreBounded(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"message":"Bad Gateway."}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, WithMaxRetries(2))
	err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains", nil, nil)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("doJSON() error = %v, want ErrProviderUnavailable", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("upstream calls = %d, want 3 (first attempt plus two retries)", got)
	}
}

// TestRetryStopsOnCanceledContext makes sure a caller that gave up is not
// kept waiting by the backoff loop.
func TestRetryStopsOnCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// An hour between retries: without the context check the call would sit
	// here long past the test's own deadline.
	client := newTestClient(t, srv,
		WithBackOff(func() backoff.BackOff { return backoff.NewConstantBackOff(time.Hour) }),
		WithMaxRetries(10),
	)

	done := make(chan error, 1)
	go func() {
		done <- client.doJSON(ctx, creds(), http.MethodGet, "domains", nil, nil)
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("doJSON() error = nil, want the failure that was in flight when the context was canceled")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("doJSON() did not return after its context was canceled")
	}
}

// TestDecodesEnvelopeData confirms the response envelope is unwrapped: the
// CDN API wraps every payload in {"message","data"} (or a paginated variant),
// and callers above this client work with the payload, not the envelope.
func TestDecodesEnvelopeData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domains/example.com" {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/domains/example.com")
		}
		_, _ = w.Write([]byte(`{"message":"ok","data":{"id":"uuid-1","name":"example.com"}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := client.doJSON(context.Background(), creds(), http.MethodGet, "domains/example.com", nil, &out); err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}
	if out.ID != "uuid-1" || out.Name != "example.com" {
		t.Errorf("decoded payload = %+v, want the envelope's data unwrapped", out)
	}
}

// TestMessageOnlyResponseIsNotAnError covers the deletes and toggles that
// answer with a bare {"message": "..."}: no data is not a failure.
func TestMessageOnlyResponseIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"message":"The domain was deleted successfully."}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	var out struct {
		ID string `json:"id"`
	}
	if err := client.doJSON(context.Background(), creds(), http.MethodDelete, "domains/example.com", nil, &out); err != nil {
		t.Fatalf("doJSON() error = %v, want nil for a message-only response", err)
	}
	if out.ID != "" {
		t.Errorf("out = %+v, want it left untouched when the response carries no data", out)
	}
}

// TestRequestShape pins the headers and the URL the adapter sends, so a
// change to either shows up here rather than as a 403 from the edge.
func TestRequestShape(t *testing.T) {
	var (
		method      string
		path        string
		accept      string
		contentType string
		userAgent   string
		body        []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		accept = r.Header.Get("Accept")
		contentType = r.Header.Get("Content-Type")
		userAgent = r.Header.Get("User-Agent")
		body, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	err := client.doJSON(context.Background(), creds(), http.MethodPost, "/domains/dns-service",
		map[string]any{"domain": "example.com", "domain_type": "full"}, nil)
	if err != nil {
		t.Fatalf("doJSON() error = %v", err)
	}

	if method != http.MethodPost {
		t.Errorf("method = %q, want POST", method)
	}
	// A leading slash on the path argument must not produce "//domains/...".
	if path != "/domains/dns-service" {
		t.Errorf("path = %q, want %q", path, "/domains/dns-service")
	}
	if accept != "application/json" {
		t.Errorf("Accept = %q, want application/json", accept)
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
	if userAgent != defaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", userAgent, defaultUserAgent)
	}
	if !strings.Contains(string(body), `"domain":"example.com"`) {
		t.Errorf("request body = %q, want the marshaled payload", body)
	}
}

// TestRedactedHeadersHideTheKey guards the debug log: it gets pasted into
// issues, so it may show the scheme and a length, never the key.
func TestRedactedHeadersHideTheKey(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Apikey "+realKey)
	header.Set("Accept", "application/json")

	got := redactedHeaders(header)
	if strings.Contains(got["Authorization"], realKey) {
		t.Errorf("Authorization = %q, want the key redacted", got["Authorization"])
	}
	if !strings.HasPrefix(got["Authorization"], "Apikey ") {
		t.Errorf("Authorization = %q, want the scheme kept so a wrong one is visible", got["Authorization"])
	}
	if got["Accept"] != "application/json" {
		t.Errorf("Accept = %q, want it passed through", got["Accept"])
	}
}

// TestMissingCredentialFailsBeforeTheRequest checks that an empty key is
// rejected locally: blaming the provider for a missing credential sends the
// caller looking in the wrong place.
func TestMissingCredentialFailsBeforeTheRequest(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Add(1)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	err := client.doJSON(context.Background(), domain.ProviderCredentials{}, http.MethodGet, "domains", nil, nil)
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("doJSON() error = %v, want ErrInvalidCredentials", err)
	}
	if called.Load() != 0 {
		t.Error("the request was sent despite there being no credential to send")
	}
}

// equalDetails compares two ProviderError.Details maps.
func equalDetails(got, want map[string][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for field, wantMessages := range want {
		gotMessages, ok := got[field]
		if !ok || len(gotMessages) != len(wantMessages) {
			return false
		}
		for i := range wantMessages {
			if gotMessages[i] != wantMessages[i] {
				return false
			}
		}
	}
	return true
}
