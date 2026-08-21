package arvancloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/core/domain"
)

// TestListArvanCloudHealthChecks pins the request shape and response parsing
// of GET /domains/{domain}/health-checks, including a TCP-typed check's
// request_config.
func TestListArvanCloudHealthChecks(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[
			{"id":"hc-1","name":"db-check","origin":"pool-1","origin_type":"pool","interval":30000,
				"threshold":3,"type":"TCP","status":true,"retries":2,
				"request_config":{"port":5432,"timeout":3000}}
		]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	checks, err := provider.ListArvanCloudHealthChecks(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudHealthChecks() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/health-checks" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/health-checks", records)
	}
	if len(checks) != 1 || checks[0].ID != "hc-1" || checks[0].Type != domain.ArvanCloudHealthCheckTCP {
		t.Fatalf("checks = %+v, want the parsed TCP check", checks)
	}
	if checks[0].RequestConfig.TCP == nil || checks[0].RequestConfig.TCP.Port != 5432 || checks[0].RequestConfig.TCP.TimeoutMS != 3000 {
		t.Errorf("checks[0].RequestConfig.TCP = %+v, want port=5432 timeout_ms=3000", checks[0].RequestConfig.TCP)
	}
	if checks[0].RequestConfig.HTTP != nil {
		t.Errorf("checks[0].RequestConfig.HTTP = %+v, want nil for a TCP check", checks[0].RequestConfig.HTTP)
	}
}

// TestCreateArvanCloudHealthCheckTCP pins the request body of POST
// /domains/{domain}/health-checks for a TCP check: request_config carries
// only port/timeout.
func TestCreateArvanCloudHealthCheckTCP(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"hc-1","name":"db-check","origin":"pool-1","origin_type":"pool",
			"interval":30000,"threshold":3,"type":"TCP","status":true,
			"request_config":{"port":5432,"timeout":3000}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	hc := domain.ArvanCloudHealthCheck{
		Name: "db-check", Origin: "pool-1", OriginType: domain.ArvanCloudHealthCheckOriginTypePool,
		IntervalMS: 30000, Threshold: 3, Type: domain.ArvanCloudHealthCheckTCP, Status: true,
		RequestConfig: domain.ArvanCloudHealthCheckRequestConfig{
			TCP: &domain.ArvanCloudHealthCheckTCPConfig{Port: 5432, TimeoutMS: 3000},
		},
	}
	created, err := provider.CreateArvanCloudHealthCheck(context.Background(), creds(), "example.com", hc)
	if err != nil {
		t.Fatalf("CreateArvanCloudHealthCheck() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPost || records[0].path != "/domains/example.com/health-checks" {
		t.Fatalf("request = %+v, want a single POST /domains/example.com/health-checks", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if body["type"] != "TCP" || body["interval"] != float64(30000) {
		t.Errorf("request body = %+v, want type=TCP interval=30000", body)
	}
	rc, ok := body["request_config"].(map[string]any)
	if !ok || rc["port"] != float64(5432) || rc["timeout"] != float64(3000) {
		t.Errorf("request body[request_config] = %+v, want port=5432 timeout=3000", body["request_config"])
	}
	if _, hasMethod := rc["method"]; hasMethod {
		t.Errorf("request body[request_config] = %+v, must not carry HTTP-only fields for a TCP check", rc)
	}
	if created.ID != "hc-1" {
		t.Errorf("created.ID = %q, want %q", created.ID, "hc-1")
	}
}

// TestCreateArvanCloudHealthCheckHTTP pins the request body of POST
// /domains/{domain}/health-checks for an HTTP check, including
// expected_response and sent_headers.
func TestCreateArvanCloudHealthCheckHTTP(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"hc-2","name":"web-check","origin":"pool-1","origin_type":"pool",
			"interval":60000,"threshold":2,"type":"HTTP","status":false,
			"request_config":{"method":"GET","port":80,"path":"/healthz","allow_insecure":false,
				"expected_response":{"codes":[200]},"timeout":5000}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	hc := domain.ArvanCloudHealthCheck{
		Name: "web-check", Origin: "pool-1", OriginType: domain.ArvanCloudHealthCheckOriginTypePool,
		IntervalMS: 60000, Threshold: 2, Type: domain.ArvanCloudHealthCheckHTTP, Status: false,
		RequestConfig: domain.ArvanCloudHealthCheckRequestConfig{
			HTTP: &domain.ArvanCloudHealthCheckHTTPConfig{
				Method: domain.ArvanCloudHealthCheckHTTPMethodGet, Port: 80, Path: "/healthz",
				ExpectedResponse: domain.ArvanCloudHealthCheckExpectedResponse{Codes: []int{200}},
				TimeoutMS:        5000,
				SentHeaders:      []domain.ArvanCloudHealthCheckSentHeader{{Key: "X-Probe", Value: "1"}},
			},
		},
	}
	created, err := provider.CreateArvanCloudHealthCheck(context.Background(), creds(), "example.com", hc)
	if err != nil {
		t.Fatalf("CreateArvanCloudHealthCheck() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != false {
		t.Errorf(`request body["status"] = %#v, hasKey=%v, want an explicit false`, status, hasStatus)
	}
	rc, ok := body["request_config"].(map[string]any)
	if !ok || rc["method"] != "GET" || rc["path"] != "/healthz" {
		t.Errorf("request body[request_config] = %+v, want method=GET path=/healthz", body["request_config"])
	}
	expected, ok := rc["expected_response"].(map[string]any)
	if !ok || len(expected["codes"].([]any)) != 1 {
		t.Errorf("request body[request_config][expected_response] = %+v, want codes=[200]", rc["expected_response"])
	}
	sentHeaders, ok := rc["sent_headers"].([]any)
	if !ok || len(sentHeaders) != 1 {
		t.Errorf("request body[request_config][sent_headers] = %+v, want one entry", rc["sent_headers"])
	}
	if created.Type != domain.ArvanCloudHealthCheckHTTP || created.RequestConfig.HTTP == nil {
		t.Errorf("created = %+v, want the parsed HTTP check", created)
	}
}

// TestGetArvanCloudHealthCheck pins the request shape and response parsing
// of GET /domains/{domain}/health-checks/{id}.
func TestGetArvanCloudHealthCheck(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"hc-1","name":"db-check","origin":"pool-1","origin_type":"pool",
			"interval":30000,"threshold":3,"type":"TCP","status":true,"request_config":{"port":5432,"timeout":3000}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	hc, err := provider.GetArvanCloudHealthCheck(context.Background(), creds(), "example.com", "hc-1")
	if err != nil {
		t.Fatalf("GetArvanCloudHealthCheck() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/health-checks/hc-1" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/health-checks/hc-1", records)
	}
	if hc.ID != "hc-1" || hc.Threshold != 3 {
		t.Errorf("hc = %+v, want the parsed check", hc)
	}
}

// TestUpdateArvanCloudHealthCheck pins the request body of PATCH
// /domains/{domain}/health-checks/{id}.
func TestUpdateArvanCloudHealthCheck(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":{"id":"hc-1","name":"db-check","origin":"pool-1","origin_type":"pool",
			"interval":60000,"threshold":5,"type":"TCP","status":false,"request_config":{"port":5432,"timeout":3000}}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	hc := domain.ArvanCloudHealthCheck{
		Name: "db-check", Origin: "pool-1", OriginType: domain.ArvanCloudHealthCheckOriginTypePool,
		IntervalMS: 60000, Threshold: 5, Type: domain.ArvanCloudHealthCheckTCP, Status: false,
		RequestConfig: domain.ArvanCloudHealthCheckRequestConfig{
			TCP: &domain.ArvanCloudHealthCheckTCPConfig{Port: 5432, TimeoutMS: 3000},
		},
	}
	updated, err := provider.UpdateArvanCloudHealthCheck(context.Background(), creds(), "example.com", "hc-1", hc)
	if err != nil {
		t.Fatalf("UpdateArvanCloudHealthCheck() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodPatch || records[0].path != "/domains/example.com/health-checks/hc-1" {
		t.Fatalf("request = %+v, want a single PATCH /domains/example.com/health-checks/hc-1", records)
	}
	var body map[string]any
	if err := json.Unmarshal(records[0].body, &body); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != false {
		t.Errorf(`request body["status"] = %#v, hasKey=%v, want an explicit false`, status, hasStatus)
	}
	if updated.Threshold != 5 {
		t.Errorf("updated.Threshold = %d, want 5", updated.Threshold)
	}
}

// TestDeleteArvanCloudHealthCheck pins the request shape of DELETE
// /domains/{domain}/health-checks/{id}.
func TestDeleteArvanCloudHealthCheck(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"message":"Deleted successfully"}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	if err := provider.DeleteArvanCloudHealthCheck(context.Background(), creds(), "example.com", "hc-1"); err != nil {
		t.Fatalf("DeleteArvanCloudHealthCheck() error = %v", err)
	}
	if len(records) != 1 || records[0].method != http.MethodDelete || records[0].path != "/domains/example.com/health-checks/hc-1" {
		t.Fatalf("request = %+v, want a single DELETE /domains/example.com/health-checks/hc-1", records)
	}
}

// TestDeleteArvanCloudHealthCheckNotFound proves a 404 surfaces as
// domain.ErrNotFound, consistent with the tolerant-delete contract at the
// use-case layer (app.DeleteArvanCloudHealthCheck).
func TestDeleteArvanCloudHealthCheckNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	provider := newTestProvider(t, srv)
	err := provider.DeleteArvanCloudHealthCheck(context.Background(), creds(), "example.com", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteArvanCloudHealthCheck() error = %v, want domain.ErrNotFound", err)
	}
}

// TestListArvanCloudDomainHealthCheckZones pins the request shape of GET
// /domains/{domain}/health-checks/zones (the domain-scoped, "regions"
// operationId endpoint).
func TestListArvanCloudDomainHealthCheckZones(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"fra","name":"Frankfurt"},{"id":"lah","name":"Tehran"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	zones, err := provider.ListArvanCloudDomainHealthCheckZones(context.Background(), creds(), "example.com")
	if err != nil {
		t.Fatalf("ListArvanCloudDomainHealthCheckZones() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/health-checks/zones" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/health-checks/zones", records)
	}
	if len(zones) != 2 || zones[0].ID != "fra" {
		t.Errorf("zones = %+v, want the two parsed zones", zones)
	}
}

// TestListArvanCloudHealthCheckZones pins the request shape of the global,
// account-independent GET /health-checks/zones — no /domains/{domain}
// prefix, unlike every other endpoint in this file.
func TestListArvanCloudHealthCheckZones(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"id":"fra","name":"Frankfurt"}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	zones, err := provider.ListArvanCloudHealthCheckZones(context.Background(), creds())
	if err != nil {
		t.Fatalf("ListArvanCloudHealthCheckZones() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/health-checks/zones" {
		t.Fatalf("request = %+v, want a single GET /health-checks/zones", records)
	}
	if len(zones) != 1 || zones[0].Name != "Frankfurt" {
		t.Errorf("zones = %+v, want the one parsed zone", zones)
	}
}

// TestGetArvanCloudHealthCheckSummary pins the query parameters sent to GET
// /domains/{domain}/health-checks/reports/summary and the response parsing.
func TestGetArvanCloudHealthCheckSummary(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"zone":"fra","status":true,"total":100,"failed":1,
			"details":[{"date":"2026-08-21T00:00:00Z","status":true}]}]}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	query := domain.ArvanCloudHealthCheckReportQuery{Name: "db-check", Upstream: "1.1.1.1", Period: "1h", Direction: "desc"}
	report, err := provider.GetArvanCloudHealthCheckSummary(context.Background(), creds(), "example.com", query)
	if err != nil {
		t.Fatalf("GetArvanCloudHealthCheckSummary() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/health-checks/reports/summary" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/health-checks/reports/summary", records)
	}
	q := records[0].query
	if want := "direction=desc&name=db-check&period=1h&upstream=1.1.1.1"; q != want {
		t.Errorf("request query = %q, want %q", q, want)
	}
	if len(report) != 1 || report[0].Zone != "fra" || len(report[0].Details) != 1 {
		t.Errorf("report = %+v, want the one parsed zone summary", report)
	}
}

// TestGetArvanCloudHealthCheckDetails pins the query parameters (including
// type/per_page/page, only sent by this endpoint) and proves the paginated
// {"data":..., "meta":...} response shape — which is NOT the {"data":...}
// envelope every other endpoint in this adapter uses — decodes correctly.
func TestGetArvanCloudHealthCheckDetails(t *testing.T) {
	var records []requestRecord
	srv := recordingServer(t, 0, func(*http.Request) []byte {
		return []byte(`{"data":[{"date":"2026-08-21T00:00:00Z","zone":"fra","upstream":"1.1.1.1","status":false,"message":"timeout"}],
			"meta":{"current_page":1,"from":1,"last_page":3,"per_page":25,"to":1,"total":75}}`)
	}, &records)
	defer srv.Close()

	provider := newTestProvider(t, srv)
	query := domain.ArvanCloudHealthCheckReportQuery{Name: "db-check", Upstream: "1.1.1.1", Type: "error", PerPage: 25, Page: 1}
	details, meta, err := provider.GetArvanCloudHealthCheckDetails(context.Background(), creds(), "example.com", query)
	if err != nil {
		t.Fatalf("GetArvanCloudHealthCheckDetails() error = %v", err)
	}

	if len(records) != 1 || records[0].method != http.MethodGet || records[0].path != "/domains/example.com/health-checks/reports/details" {
		t.Fatalf("request = %+v, want a single GET /domains/example.com/health-checks/reports/details", records)
	}
	q := records[0].query
	if want := "name=db-check&page=1&per_page=25&type=error&upstream=1.1.1.1"; q != want {
		t.Errorf("request query = %q, want %q", q, want)
	}
	if len(details) != 1 || details[0].Message != "timeout" {
		t.Errorf("details = %+v, want the one parsed probe result", details)
	}
	if meta.LastPage != 3 || meta.Total != 75 {
		t.Errorf("meta = %+v, want last_page=3 total=75", meta)
	}
}
