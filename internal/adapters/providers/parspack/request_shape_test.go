package parspack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// testToken stands in for a real Parspack key. A real one may be supplied
// through PARSPACK_API_KEY to run this against the live API; none is ever
// committed here.
func testToken() string {
	if key := os.Getenv("PARSPACK_API_KEY"); key != "" {
		return key
	}
	return "0000|placeholder-token-for-tests"
}

// capturedRequest is what the fake provider saw.
type capturedRequest struct {
	method string
	path   string
	header http.Header
}

// TestOutboundRequestShape pins the outgoing request against the request
// shape Parspack is known to accept, header for header.
//
// Every assertion here corresponds to a way this has actually broken:
// the Authorization prefix, the Content-Type that must be present even on a
// bodyless GET, and above all the User-Agent — net/http supplies
// "Go-http-client/1.1" whenever the header is unset, and Parspack's WAF
// answers that specific value with a 403 "IP Blocked" page while accepting
// the identical request under any other User-Agent. A future refactor that
// drops the explicit header would reintroduce a failure that looks like a
// credential problem and is not one.
func TestOutboundRequestShape(t *testing.T) {
	var got capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = capturedRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone()}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"ok","data":[]}`))
	}))
	defer srv.Close()

	client, err := New(WithCDNBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	token := testToken()
	if _, err := client.ListCDNZones(context.Background(), domain.ProviderCredentials{APIKey: token}); err != nil {
		t.Fatalf("ListCDNZones() error = %v", err)
	}

	if got.method != http.MethodGet {
		t.Errorf("method = %q, want GET", got.method)
	}
	if want := "/" + zonesBasePath; got.path != want {
		t.Errorf("path = %q, want %q", got.path, want)
	}

	wantHeaders := map[string]string{
		"Content-Type":    "application/json",
		"Accept":          "application/json",
		"Accept-Language": "en",
		"Authorization":   "Bearer " + token,
	}
	for name, want := range wantHeaders {
		if got := got.header.Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}

	ua := got.header.Get("User-Agent")
	if ua == "" {
		t.Error("User-Agent is empty; net/http would substitute its default, which Parspack blocks")
	}
	if strings.HasPrefix(ua, "Go-http-client") {
		t.Errorf("User-Agent = %q, the value Parspack's WAF answers with 403 IP Blocked", ua)
	}
	if !strings.Contains(ua, "do0ps") {
		t.Errorf("User-Agent = %q, want it to identify this client", ua)
	}
}

// TestOutboundRequestShapeIsSharedAcrossSurfaces proves the shape comes from
// the one request builder every tool goes through, not from list_cdn_zones.
// A surface or method that bypassed doJSONBase would fail here.
func TestOutboundRequestShapeIsSharedAcrossSurfaces(t *testing.T) {
	var got capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = capturedRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone()}
		w.Header().Set("Content-Type", "application/json")
		// Satisfies the cloud-server, CDN and SSL decoders alike: each reads
		// only the field it cares about.
		_, _ = w.Write([]byte(`{"success":true,"message":"ok","data":[],"vms":[],"ssh_keys":[]}`))
	}))
	defer srv.Close()

	client, err := New(WithBaseURL(srv.URL), WithCDNBaseURL(srv.URL), WithSSLBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	creds := domain.ProviderCredentials{APIKey: testToken()}
	calls := map[string]func() error{
		"cloud-server (list_servers)": func() error {
			_, err := client.ListServers(context.Background(), creds)
			return err
		},
		"cloud-server (list_ssh_keys)": func() error {
			_, err := client.ListSSHKeys(context.Background(), creds)
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
			got = capturedRequest{}
			if err := call(); err != nil {
				t.Fatalf("call error = %v", err)
			}

			for _, name := range []string{"Content-Type", "Accept", "Accept-Language", "User-Agent"} {
				if got.header.Get(name) == "" {
					t.Errorf("header %s is missing", name)
				}
			}
			if ua := got.header.Get("User-Agent"); strings.HasPrefix(ua, "Go-http-client") {
				t.Errorf("User-Agent = %q, the value Parspack's WAF blocks", ua)
			}
			if auth := got.header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
				t.Errorf("Authorization = %q, want a single %q prefix", auth, "Bearer ")
			}
		})
	}
}

// TestWithUserAgentRejectsEmpty guards the override: accepting an empty
// user agent would silently hand the request back to net/http's default.
func TestWithUserAgentRejectsEmpty(t *testing.T) {
	if _, err := New(WithUserAgent("  ")); err == nil {
		t.Error("New(WithUserAgent(\"  \")) succeeded, want an error")
	}
}
