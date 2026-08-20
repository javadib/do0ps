package parspack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/parspack"
	"github.com/javadib/do0ps/internal/core/domain"
)

// newTestCDNClient wires a Client whose CDN base URL points at a fake CDN
// server, while the cloud-server base URL stays at its (unused) default —
// proving the two surfaces really are configured independently (AGENTS.md
// 4.5).
func newTestCDNClient(t *testing.T, handler http.HandlerFunc) *parspack.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := parspack.New(parspack.WithCDNBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestListCDNZonesSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones" {
			t.Errorf("path = %s, want /external/api/v1/zones", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"id":"Q5rGXaKV","uuid":"zone-1","target_domain":"example.com","status":"active","plan":"free","expire_at":"2024-01-13"}
		]}`))
	})

	zones, err := c.ListCDNZones(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListCDNZones: %v", err)
	}
	if len(zones) != 1 || zones[0].UUID != "zone-1" || zones[0].Domain != "example.com" {
		t.Errorf("zones = %+v, want a single example.com zone with UUID zone-1", zones)
	}
	if zones[0].Status != "active" {
		t.Errorf("status = %q, want active", zones[0].Status)
	}
}

func TestGetCDNZoneSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/zones/zone-1" {
			t.Errorf("path = %s, want /external/api/v1/zones/zone-1", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"target_domain":"example.com","status":"active","plan":"standard","proxy":true,
			"ns_status":true,"billing_cycle":"monthly","remaining_days":60
		}}`))
	})

	zone, err := c.GetCDNZone(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetCDNZone: %v", err)
	}
	if zone.UUID != "zone-1" || zone.Domain != "example.com" || zone.RemainingDays != 60 {
		t.Errorf("zone = %+v, want UUID zone-1, domain example.com, remaining_days 60", zone)
	}
	if !zone.Proxy || !zone.NSStatus {
		t.Errorf("zone = %+v, want proxy and ns_status both true", zone)
	}
}

func TestGetCDNZoneNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	_, err := c.GetCDNZone(context.Background(), creds, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

func TestDeleteCDNZoneSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteCDNZone(context.Background(), creds, "zone-1"); err != nil {
		t.Fatalf("DeleteCDNZone: %v", err)
	}
}

func TestListCDNZonePlansSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/orders/packages" {
			t.Errorf("path = %s, want /external/api/v1/orders/packages", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"package_periods":[{"title":"monthly"}],
			"packages":[{"plan":"free","prices":{"currency":"T","monthly":150000,"quarterly":0,"semiannually":0,"annually":0}}]
		}}`))
	})

	plans, err := c.ListCDNZonePlans(context.Background(), creds)
	if err != nil {
		t.Fatalf("ListCDNZonePlans: %v", err)
	}
	if len(plans) != 1 || plans[0].Plan != "free" || plans[0].Monthly != 150000 {
		t.Errorf("plans = %+v, want a single free plan priced at 150000 monthly", plans)
	}
}

func TestGetNameserverRecordsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v1/orders/zone-1/ns-records" {
			t.Errorf("path = %s, want /external/api/v1/orders/zone-1/ns-records", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"ns1":"ns1.example.net","ns2":"ns2.example.net","ns3":"","ns4":"",
			"current_ns":["ns1.registrar.net","ns2.registrar.net"]
		}}`))
	})

	ns, err := c.GetNameserverRecords(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("GetNameserverRecords: %v", err)
	}
	if ns.NS1 != "ns1.example.net" || len(ns.CurrentNS) != 2 {
		t.Errorf("ns = %+v, want ns1.example.net and 2 current nameservers", ns)
	}
}

func TestCreateCDNZoneSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v2/orders" {
			t.Errorf("path = %s, want /external/api/v2/orders", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["domain"] != "example.com" || body["plan"] != "free" || body["billing_cycle"] != "free" {
			t.Errorf("body = %+v, want domain/plan/billing_cycle example.com/free/free", body)
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":{
			"zone_id":"zone-1","ns_records":{"ns1":"ns1.example.net","ns2":"ns2.example.net","ns3":"","ns4":""},
			"status":"active","warnings":[]
		}}`))
	})

	zone, err := c.CreateCDNZone(context.Background(), creds, domain.CDNZoneSpec{
		Domain: "example.com", Plan: "free", BillingCycle: "free",
	})
	if err != nil {
		t.Fatalf("CreateCDNZone: %v", err)
	}
	if zone.UUID != "zone-1" || zone.Status != "active" {
		t.Errorf("zone = %+v, want UUID zone-1 and status active", zone)
	}
}

func TestCreateCDNZoneValidationError(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The domain parameter is required.","errors":{"domain":["The domain is required."]}}`))
	})

	_, err := c.CreateCDNZone(context.Background(), creds, domain.CDNZoneSpec{Plan: "free", BillingCycle: "free"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestListDNSRecordsSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/external/api/v2/zones/zone-1/dns-records" {
			t.Errorf("path = %s, want /external/api/v2/zones/zone-1/dns-records", got)
		}
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[
			{"zone":"example.com","ttl":3600,"type":"A","host":"@","proxy":"cdn-smart-caching",
			 "records":[{"content":"1.2.3.4","disabled":false,"port":null,"weight":null,"priority":null,"flags":null,"tag":null,"serial":null,"refresh":null,"minimum":null}]},
			{"zone":"example.com","ttl":3600,"type":"NS","host":"@","proxy":null,
			 "records":[{"content":"ns1.example.org.","disabled":false},{"content":"ns2.example.org.","disabled":false}]}
		]}`))
	})

	records, err := c.ListDNSRecords(context.Background(), creds, "zone-1")
	if err != nil {
		t.Fatalf("ListDNSRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Type != domain.DNSRecordTypeA || records[0].Values[0].Content != "1.2.3.4" {
		t.Errorf("records[0] = %+v, want type A with content 1.2.3.4", records[0])
	}
	if records[1].Type != domain.DNSRecordTypeNS || len(records[1].Values) != 2 {
		t.Errorf("records[1] = %+v, want type NS with 2 values", records[1])
	}
	for i := range records {
		if records[i].ZoneUUID != "zone-1" {
			t.Errorf("records[%d].ZoneUUID = %q, want zone-1", i, records[i].ZoneUUID)
		}
	}
}

func TestCreateDNSRecordSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/external/api/v2/zones/zone-1/dns-records" {
			t.Errorf("path = %s, want /external/api/v2/zones/zone-1/dns-records", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["host"] != "api" || body["type"] != "A" || body["ttl"] != float64(3600) {
			t.Errorf("body = %+v, want host/type/ttl api/A/3600", body)
		}
		record, ok := body["record"].(map[string]any)
		if !ok || record["content"] != "203.0.113.10" {
			t.Errorf("body.record = %+v, want content 203.0.113.10", body["record"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rec, err := c.CreateDNSRecord(context.Background(), creds, "zone-1", domain.DNSRecord{
		Host: "api", Type: domain.DNSRecordTypeA, TTL: 3600, Proxy: domain.DNSRecordProxyDirect,
		Values: []domain.DNSRecordValue{{Content: "203.0.113.10"}},
	})
	if err != nil {
		t.Fatalf("CreateDNSRecord: %v", err)
	}
	if rec.ZoneUUID != "zone-1" || rec.Host != "api" {
		t.Errorf("rec = %+v, want zone zone-1 and host api", rec)
	}
}

func TestUpdateDNSRecordSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	rec, err := c.UpdateDNSRecord(context.Background(), creds, "zone-1", domain.DNSRecord{
		Host: "api", Type: domain.DNSRecordTypeA, TTL: 300, Proxy: domain.DNSRecordProxyDirect,
		Values: []domain.DNSRecordValue{{Content: "203.0.113.20"}},
	})
	if err != nil {
		t.Fatalf("UpdateDNSRecord: %v", err)
	}
	if rec.TTL != 300 {
		t.Errorf("TTL = %d, want 300", rec.TTL)
	}
}

func TestDeleteDNSRecordSuccess(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["host"] != "api" || body["type"] != "A" {
			t.Errorf("body = %+v, want host/type api/A", body)
		}
		if _, hasRecord := body["record"]; hasRecord {
			t.Errorf("body = %+v, want no record field when content is empty (delete all values)", body)
		}

		_, _ = w.Write([]byte(`{"success":true,"message":"done","data":[]}`))
	})

	if err := c.DeleteDNSRecord(context.Background(), creds, "zone-1", "api", domain.DNSRecordTypeA, ""); err != nil {
		t.Fatalf("DeleteDNSRecord: %v", err)
	}
}

func TestDeleteDNSRecordNotFound(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"message":"Not Found","errors":[]}`))
	})

	err := c.DeleteDNSRecord(context.Background(), creds, "zone-1", "api", domain.DNSRecordTypeA, "")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

// TestOperationFailMapsToProviderUnavailable proves the CDN API's 424
// ("Operation Fail") status is treated as a provider-side failure, not a bad
// request (docs/api-specs/parspack-cdn.openapi.yaml).
func TestOperationFailMapsToProviderUnavailable(t *testing.T) {
	c := newTestCDNClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(424)
		_, _ = w.Write([]byte(`{"success":false,"message":"Operation failed!","errors":[]}`))
	})

	_, err := c.ListCDNZones(context.Background(), creds)
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("error = %v, want domain.ErrProviderUnavailable", err)
	}
}
