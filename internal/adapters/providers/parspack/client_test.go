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

// realToken is shaped like a Parspack key: an ID, a pipe, then the secret.
const realToken = "9926|aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"

// TestAuthorizationHeader covers the shapes an api_key tool argument actually
// arrives in. A chatbot passes the raw token, but a user pasting from the
// Parspack dashboard or from a curl command may include the "Bearer " prefix
// or a trailing newline; all of those must reach Parspack as one well-formed
// header.
func TestAuthorizationHeader(t *testing.T) {
	want := "Bearer " + realToken

	valid := []struct {
		name   string
		apiKey string
	}{
		{"raw token", realToken},
		{"bearer prefixed", "Bearer " + realToken},
		{"lowercase bearer prefix", "bearer " + realToken},
		{"surrounding whitespace", "  " + realToken + "  "},
		{"trailing newline from a copy-paste", realToken + "\n"},
		{"trailing CRLF from a copy-paste", realToken + "\r\n"},
		{"bearer prefixed with a trailing newline", "Bearer " + realToken + "\n"},
		{"prefix doubled by a caller that also prepends", "Bearer Bearer " + realToken},
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
		{"bearer prefix with no token", "Bearer "},
		{"embedded newline", "9926|abc\ndef"},
		{"embedded carriage return", "9926|abc\rdef"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := authorizationHeader(tc.apiKey)
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("authorizationHeader() = %q, error = %v, want ErrInvalidCredentials", got, err)
			}
			if strings.Contains(err.Error(), tc.apiKey) && tc.apiKey != "" {
				t.Errorf("error message %q echoes the credential back", err)
			}
		})
	}
}

// TestAuthorizationHeaderReachesProvider pins the normalization end to end:
// whichever shape the tool argument had, the provider sees one "Bearer " and
// the request succeeds.
func TestAuthorizationHeaderReachesProvider(t *testing.T) {
	for _, apiKey := range []string{realToken, "Bearer " + realToken, realToken + "\n"} {
		var seen string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"vms":[]}`))
		}))

		client, err := New(WithBaseURL(srv.URL))
		if err != nil {
			srv.Close()
			t.Fatalf("New() error = %v", err)
		}
		if _, err := client.ListServers(context.Background(), domain.ProviderCredentials{APIKey: apiKey}); err != nil {
			srv.Close()
			t.Fatalf("ListServers() with api_key %q: %v", apiKey, err)
		}
		if want := "Bearer " + realToken; seen != want {
			t.Errorf("api_key %q sent Authorization %q, want %q", apiKey, seen, want)
		}
		srv.Close()
	}
}

// TestAuthorizationHeaderErrorIsActionable guards the reporting path: a
// malformed api_key must surface as an invalid-credentials error naming the
// cause, not as ErrProviderUnavailable, which would blame Parspack for a
// local input problem and reach the caller as a generic tool failure.
func TestAuthorizationHeaderErrorIsActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("provider was called despite a malformed api_key")
	}))
	defer srv.Close()

	client, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = client.ListServers(context.Background(), domain.ProviderCredentials{APIKey: "9926|abc\ndef"})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("ListServers() error = %v, want ErrInvalidCredentials", err)
	}
	if errors.Is(err, domain.ErrProviderUnavailable) {
		t.Errorf("ListServers() error = %v, want the provider not to be blamed", err)
	}
	if !strings.Contains(err.Error(), "Authorization header") {
		t.Errorf("ListServers() error = %v, want it to name the failing step", err)
	}
}
