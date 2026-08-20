package parspack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestMapErrorResponse pins what a caller learns from a rejected request: the
// status Parspack answered with, whatever it said in the body, and a sentinel
// only when the status means the same thing on all three API surfaces.
func TestMapErrorResponse(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantSentinel error
		wantContains []string
	}{
		{
			name:         "403 has no sentinel but keeps the reason",
			status:       http.StatusForbidden,
			body:         `{"message":"IP not whitelisted"}`,
			wantSentinel: nil,
			wantContains: []string{"403", "IP not whitelisted"},
		},
		{
			name:         "401 classifies as bad credentials and says why",
			status:       http.StatusUnauthorized,
			body:         `{"message":"Unauthenticated."}`,
			wantSentinel: domain.ErrInvalidCredentials,
			wantContains: []string{"401", "Unauthenticated."},
		},
		{
			name:         "422 carries the per-field validation errors",
			status:       http.StatusUnprocessableEntity,
			body:         `{"message":"The given data was invalid.","errors":{"domain":["The domain is required."]}}`,
			wantSentinel: domain.ErrInvalidInput,
			wantContains: []string{"422", "The given data was invalid.", "domain", "The domain is required."},
		},
		{
			name:         "500 is the provider's fault and keeps its message",
			status:       http.StatusInternalServerError,
			body:         `{"message":"Internal error","request_id":"abc"}`,
			wantSentinel: domain.ErrProviderUnavailable,
			wantContains: []string{"500", "Internal error"},
		},
		{
			name:         "a non-JSON body is shown rather than dropped",
			status:       http.StatusBadGateway,
			body:         "<html><body>502 Bad Gateway</body></html>",
			wantSentinel: domain.ErrProviderUnavailable,
			wantContains: []string{"502", "Bad Gateway"},
		},
		{
			name:         "an empty body still reports the status",
			status:       http.StatusForbidden,
			body:         "",
			wantSentinel: nil,
			wantContains: []string{"403"},
		},
		{
			name:         "a CDN envelope error message is picked up",
			status:       statusFailedDependency,
			body:         `{"success":false,"message":"Operation Fail","data":null}`,
			wantSentinel: domain.ErrProviderUnavailable,
			wantContains: []string{"424", "Operation Fail"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := mapErrorResponse("GET", "api/public/v1/vms", tc.status, []byte(tc.body))

			if tc.wantSentinel != nil && !errors.Is(err, tc.wantSentinel) {
				t.Errorf("error = %v, want it to classify as %v", err, tc.wantSentinel)
			}

			var provErr *domain.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error = %v, want a *domain.ProviderError", err)
			}
			if provErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", provErr.StatusCode, tc.status)
			}
			if provErr.Provider != providerName {
				t.Errorf("Provider = %q, want %q", provErr.Provider, providerName)
			}

			for _, want := range tc.wantContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to mention %q", err, want)
				}
			}
		})
	}
}

// TestProviderErrorsAreUniformAcrossSurfaces guards the "fix it in the shared
// wrapper, not per tool" property: the cloud-server, CDN and SSL surfaces all
// go through doJSONBase, so a 403 on any of them reports the same way. A new
// surface that skipped it would fail here.
func TestProviderErrorsAreUniformAcrossSurfaces(t *testing.T) {
	const body = `{"message":"IP not whitelisted"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client, err := New(WithBaseURL(srv.URL), WithCDNBaseURL(srv.URL), WithSSLBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	creds := domain.ProviderCredentials{APIKey: "9926|secret-token-value"}
	calls := map[string]func() error{
		"cloud-server (list_servers)": func() error {
			_, err := client.ListServers(context.Background(), creds)
			return err
		},
		"CDN (list_cdn_zones)": func() error {
			_, err := client.ListCDNZones(context.Background(), creds)
			return err
		},
		"SSL (list_ssl_products)": func() error {
			_, err := client.ListSSLProducts(context.Background(), creds)
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()

			var provErr *domain.ProviderError
			if !errors.As(err, &provErr) {
				t.Fatalf("error = %v, want a *domain.ProviderError", err)
			}
			if provErr.StatusCode != http.StatusForbidden {
				t.Errorf("StatusCode = %d, want 403", provErr.StatusCode)
			}
			if provErr.Message != "IP not whitelisted" {
				t.Errorf("Message = %q, want the provider's message", provErr.Message)
			}
			if strings.Contains(err.Error(), "secret-token-value") {
				t.Errorf("error = %q, must never echo the credential", err)
			}
		})
	}
}

// TestTransportFailureKeepsTheReason covers the case where no response comes
// back at all: the cause must survive instead of collapsing to a bare
// "provider unavailable".
func TestTransportFailureKeepsTheReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // nothing is listening any more

	client, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = client.ListServers(context.Background(), domain.ProviderCredentials{APIKey: "9926|secret-token-value"})
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want ErrProviderUnavailable", err)
	}

	var provErr *domain.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("error = %v, want a *domain.ProviderError", err)
	}
	if provErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 for a request that got no response", provErr.StatusCode)
	}
	if provErr.Message == "" {
		t.Error("Message is empty, want the transport failure reason")
	}
	if strings.Contains(err.Error(), "secret-token-value") {
		t.Errorf("error = %q, must never echo the credential", err)
	}
}
