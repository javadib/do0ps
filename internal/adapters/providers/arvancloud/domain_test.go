package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// newTestProvider builds a Provider around a client pointed at srv, with
// retrying made instant (see newTestClient).
func newTestProvider(t *testing.T, srv *httptest.Server) *Provider {
	t.Helper()
	client := newTestClient(t, srv)
	provider, err := NewProvider(client)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	return provider
}

// requestRecord captures one request an httptest handler saw, for tests that
// assert on method, path and body shape.
type requestRecord struct {
	method string
	path   string
	query  string
	body   []byte
}

func recordingServer(t *testing.T, status int, respond func(r *http.Request) []byte, records *[]requestRecord) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*records = append(*records, requestRecord{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})
		if status != 0 {
			w.WriteHeader(status)
		}
		if respond != nil {
			_, _ = w.Write(respond(r))
		}
	}))
}

// TestListDomains pins the request shape and response parsing of GET
// /domains.
func TestListDomains(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"uuid-1","name":"example.com","plan_level":2,"type":"full","status":"active","dns_cloud":true},
			{"id":"uuid-2","domain":"legacy.example.com","plan_level":1,"type":"partial","status":"pending"}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	domains, err := provider.ListDomains(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListDomains() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains" {
		t.Fatalf("request = %+v, want a single GET /domains", records)
	}

	if len(domains) != 2 {
		t.Fatalf("len(domains) = %d, want 2", len(domains))
	}
	if domains[0].ID != "uuid-1" || domains[0].Name != "example.com" ||
		domains[0].PlanLevel != domain.ArvanCloudPlanGrowth || domains[0].Type != "full" ||
		domains[0].Status != "active" || !domains[0].DNSCloud {
		t.Errorf("domains[0] = %+v, want the parsed first entry", domains[0])
	}
	// The second entry only carries the deprecated "domain" field, not
	// "name" — toDomainDomain must fall back to it.
	if domains[1].Name != "legacy.example.com" {
		t.Errorf("domains[1].Name = %q, want the deprecated \"domain\" field's value", domains[1].Name)
	}
}

// TestCreateDomain pins the request body of POST /domains/dns-service and the
// response parsing into domain.ArvanCloudDomain.
func TestCreateDomain(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"message":"ok","data":{"id":"uuid-1","name":"example.com","plan_level":2,"type":"full","status":"initializing"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	created, err := provider.CreateDomain(context.Background(), creds(), domain.ArvanCloudDomainSpec{
		Name: "example.com", DomainType: domain.ArvanCloudDomainTypeFull,
		PlanLevel: domain.ArvanCloudPlanGrowth, ImportDNSRecords: true,
	})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/dns-service" {
		t.Fatalf("request = %+v, want a single POST /domains/dns-service", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if body["domain"] != "example.com" || body["domain_type"] != "full" || body["plan_level"] != float64(2) {
		t.Errorf("request body = %v, want domain/domain_type/plan_level set", body)
	}
	if importVal, ok := body["import_dns_records"].(bool); !ok || !importVal {
		t.Errorf(`request body["import_dns_records"] = %v, want explicit true`, body["import_dns_records"])
	}

	if created.ID != "uuid-1" || created.Name != "example.com" || created.Status != "initializing" {
		t.Errorf("created = %+v, want the parsed response", created)
	}
}

// TestCreateDomainImportDNSRecordsDefaultsToTrue proves that a spec built
// without setting ImportDNSRecords explicitly (the zero value, false) is
// still sent as an explicit field — this port never relies on the field
// being merely absent. The MCP tool boundary is what turns "the caller
// omitted import_dns_records" into "send true"; this test pins the other
// half of that contract: whatever bool this port receives is always the one
// that reaches the wire, verbatim, never omitted.
func TestCreateDomainImportDNSRecordsDefaultsToTrue(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusCreated, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"example.com"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	// ImportDNSRecords deliberately left at its zero value here; a caller
	// that wants ArvanCloud's documented "omitted means true" default must
	// resolve it to true before it reaches this port (see
	// internal/adapters/mcp/arvancloud_domain_tools.go), which is exactly
	// what the request body below must NOT show on its own.
	if _, err := provider.CreateDomain(context.Background(), creds(), domain.ArvanCloudDomainSpec{Name: "example.com"}); err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	importVal, ok := body["import_dns_records"].(bool)
	if !ok {
		t.Fatalf(`request body["import_dns_records"] missing or not a bool: %v`, body["import_dns_records"])
	}
	if importVal {
		t.Errorf("import_dns_records = true for a spec that left it at its zero value; this port must forward exactly what it was given, not apply a default of its own")
	}
}

// TestGetDomain pins the request shape and response parsing of GET
// /domains/{domain}.
func TestGetDomain(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"example.com","plan_level":1,"type":"full","status":"active","ns_keys":["h.ns.arvancloud.ir","s.ns.arvancloud.ir"],"current_ns":["ns1.registrar.com","ns2.registrar.com"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	found, err := provider.GetDomain(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("GetDomain() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com", records)
	}
	if found.ID != "uuid-1" || found.Name != "example.com" || len(found.NSKeys) != 2 || len(found.CurrentNS) != 2 {
		t.Errorf("found = %+v, want the parsed response", found)
	}
}

// TestDeleteDomain pins the two-request shape DeleteDomain needs: a GET to
// resolve the domain's id, then the DELETE carrying it as a query parameter
// (the spec's required "id" query parameter on DELETE /domains/{domain}).
func TestDeleteDomain(t *testing.T) {
	var records []requestRecord
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records = append(records, requestRecord{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery})
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"data":{"id":"uuid-1","name":"example.com"}}`))
		case http.MethodDelete:
			_, _ = w.Write([]byte(`{"message":"The domain was deleted successfully."}`))
		}
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteDomain(context.Background(), creds(), "example.com"); err != nil {
		t.Fatalf("DeleteDomain() error = %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("requests = %+v, want a GET followed by a DELETE", records)
	}
	if records[0].method != http.MethodGet || records[0].path != "/domains/example.com" {
		t.Errorf("first request = %+v, want GET /domains/example.com", records[0])
	}
	if records[1].method != http.MethodDelete || records[1].path != "/domains/example.com" {
		t.Errorf("second request = %+v, want DELETE /domains/example.com", records[1])
	}
	if records[1].query != "id=uuid-1" {
		t.Errorf("delete query = %q, want %q", records[1].query, "id=uuid-1")
	}
}

// TestDeleteDomainNotFound is the AC #62 pins: deleting a domain the
// provider no longer has must report domain.ErrNotFound, not succeed
// silently — the same tolerant-delete contract as DeleteServer/DeleteCDNZone
// (ports.ArvanCloudProvider.DeleteDomain, AGENTS.md 4.4).
func TestDeleteDomainNotFound(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusNotFound, func(*http.Request) []byte {
		return []byte(`{"status":false,"message":"Domain not found."}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteDomain(context.Background(), creds(), "gone.example.com")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteDomain() error = %v, want domain.ErrNotFound", err)
	}

	// The lookup itself already answered 404, so DeleteDomain must not have
	// gone on to attempt the DELETE call at all.
	if len(records) != 1 || records[0].method != http.MethodGet {
		t.Errorf("requests = %+v, want only the GET that discovered the domain is gone", records)
	}
}

// TestSetNSKeys pins the request body of PUT /domains/{domain}/ns-keys and
// the partial-resource response parsing.
func TestSetNSKeys(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"ok","data":{"ns_keys":["h.ns.arvancloud.ir","s.ns.arvancloud.ir"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.SetNSKeys(context.Background(), creds(), "example.com", []string{"h.ns.arvancloud.ir", "s.ns.arvancloud.ir"})
	if err != nil {
		t.Fatalf("SetNSKeys() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/example.com/ns-keys" {
		t.Fatalf("request = %+v, want a single PUT /domains/example.com/ns-keys", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if nsKeys, ok := body["ns_keys"].([]any); !ok || len(nsKeys) != 2 {
		t.Errorf(`request body["ns_keys"] = %v, want a 2-element array`, body["ns_keys"])
	}

	if updated.Name != "example.com" || len(updated.NSKeys) != 2 {
		t.Errorf("updated = %+v, want the domain name filled in and the returned ns_keys", updated)
	}
}

// TestResetNSKeys pins the request shape of DELETE
// /domains/{domain}/ns-keys.
func TestResetNSKeys(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"ns_keys":["ns1.arvancloud.ir","ns2.arvancloud.ir"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	reset, err := provider.ResetNSKeys(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ResetNSKeys() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/ns-keys" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/ns-keys", records)
	}
	if len(reset.NSKeys) != 2 {
		t.Errorf("reset.NSKeys = %v, want 2 entries", reset.NSKeys)
	}
}

// TestCheckNSStatus pins the request shape of GET
// /domains/{domain}/ns-keys/check and the ns_domain/ns_keys mapping onto
// CurrentNS/NSKeys.
func TestCheckNSStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"ns_domain":["ns1.registrar.com","ns2.registrar.com"],"ns_keys":["h.ns.arvancloud.ir","s.ns.arvancloud.ir"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	status, err := provider.CheckNSStatus(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("CheckNSStatus() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/ns-keys/check" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/ns-keys/check", records)
	}
	if len(status.CurrentNS) != 2 || status.CurrentNS[0] != "ns1.registrar.com" {
		t.Errorf("status.CurrentNS = %v, want the ns_domain field", status.CurrentNS)
	}
	if len(status.NSKeys) != 2 || status.NSKeys[0] != "h.ns.arvancloud.ir" {
		t.Errorf("status.NSKeys = %v, want the ns_keys field", status.NSKeys)
	}
}

// TestUseOptionalNSKeys pins the request shape of POST
// /domains/{domain}/ns-keys/use-optional-keys.
func TestUseOptionalNSKeys(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"ns_keys":["alt1.arvancloud.ir","alt2.arvancloud.ir"]}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.UseOptionalNSKeys(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("UseOptionalNSKeys() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/ns-keys/use-optional-keys" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/ns-keys/use-optional-keys", records)
	}
	if len(updated.NSKeys) != 2 || updated.NSKeys[0] != "alt1.arvancloud.ir" {
		t.Errorf("updated.NSKeys = %v, want the optional key set", updated.NSKeys)
	}
}

// TestSetCnameTarget pins the request body of PUT
// /domains/{domain}/cname-setup/custom and the full-resource response
// parsing.
func TestSetCnameTarget(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"sub.example.com","type":"partial","status":"active","custom_cname":"custom.cdn.example.net"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	updated, err := provider.SetCnameTarget(context.Background(), creds(), "sub.example.com", "custom.cdn.example.net")
	if err != nil {
		t.Fatalf("SetCnameTarget() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPut || records[0].path != "/domains/sub.example.com/cname-setup/custom" {
		t.Fatalf("request = %+v, want a single PUT /domains/sub.example.com/cname-setup/custom", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if body["address"] != "custom.cdn.example.net" {
		t.Errorf(`request body["address"] = %v, want "custom.cdn.example.net"`, body["address"])
	}

	if updated.CustomCname != "custom.cdn.example.net" || updated.Type != "partial" {
		t.Errorf("updated = %+v, want the parsed response", updated)
	}
}

// TestResetCnameTarget pins the request shape of DELETE
// /domains/{domain}/cname-setup/custom.
func TestResetCnameTarget(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"sub.example.com","type":"partial","cname_target":"cdn.arvancloud.ir"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	reset, err := provider.ResetCnameTarget(context.Background(), creds(), "sub.example.com")
	if err != nil {
		t.Fatalf("ResetCnameTarget() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/sub.example.com/cname-setup/custom" {
		t.Fatalf("request = %+v, want a single DELETE /domains/sub.example.com/cname-setup/custom", records)
	}
	if reset.CnameTarget != "cdn.arvancloud.ir" {
		t.Errorf("reset.CnameTarget = %q, want the default cname_target", reset.CnameTarget)
	}
}

// TestConvertToCnameSetup pins the request shape of POST
// /domains/{domain}/cname-setup/convert.
func TestConvertToCnameSetup(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"sub.example.com","type":"partial","status":"pending"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	converted, err := provider.ConvertToCnameSetup(context.Background(), creds(), "sub.example.com")
	if err != nil {
		t.Fatalf("ConvertToCnameSetup() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/sub.example.com/cname-setup/convert" {
		t.Fatalf("request = %+v, want a single POST /domains/sub.example.com/cname-setup/convert", records)
	}
	if converted.Type != "partial" {
		t.Errorf("converted.Type = %q, want %q", converted.Type, "partial")
	}
}

// TestCheckCnameStatus pins the request shape of GET
// /domains/{domain}/cname-setup/check.
func TestCheckCnameStatus(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"uuid-1","name":"sub.example.com","type":"partial","status":"active"}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	status, err := provider.CheckCnameStatus(context.Background(), creds(), "sub.example.com")
	if err != nil {
		t.Fatalf("CheckCnameStatus() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/sub.example.com/cname-setup/check" {
		t.Fatalf("request = %+v, want a single GET /domains/sub.example.com/cname-setup/check", records)
	}
	if status.Status != "active" {
		t.Errorf("status.Status = %q, want %q", status.Status, "active")
	}
}

// TestCloneDomainConfig pins the request body of POST
// /domains/{domain}/clone.
func TestCloneDomainConfig(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"ok"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.CloneDomainConfig(context.Background(), creds(), "new.example.com", "template.example.com"); err != nil {
		t.Fatalf("CloneDomainConfig() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/new.example.com/clone" {
		t.Fatalf("request = %+v, want a single POST /domains/new.example.com/clone", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("decoding request body: %v", err)
	}
	if body["from"] != "template.example.com" {
		t.Errorf(`request body["from"] = %v, want "template.example.com"`, body["from"])
	}
}

// TestRegenerateDomainConfig pins the request shape of POST
// /domains/{domain}/regenerate.
func TestRegenerateDomainConfig(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, http.StatusAccepted, func(*http.Request) []byte {
		return []byte(`{"message":"ok"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.RegenerateDomainConfig(context.Background(), creds(), "example.com"); err != nil {
		t.Fatalf("RegenerateDomainConfig() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/regenerate" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/regenerate", records)
	}
}

// TestHoldDomain pins the request shape of POST /domains/{domain}/hold.
func TestHoldDomain(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"ok"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.HoldDomain(context.Background(), creds(), "example.com"); err != nil {
		t.Fatalf("HoldDomain() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/hold" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/hold", records)
	}
}

// TestUnholdDomain pins the request shape of POST /domains/{domain}/unhold.
func TestUnholdDomain(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"ok"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.UnholdDomain(context.Background(), creds(), "example.com"); err != nil {
		t.Fatalf("UnholdDomain() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/unhold" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/unhold", records)
	}
}
