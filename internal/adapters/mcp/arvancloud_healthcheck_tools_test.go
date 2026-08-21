package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/arvancloud"
	"github.com/javadib/do0ps/internal/core/app"
)

// newArvanCloudHealthCheckProvider wires an arvancloud.Provider onto a fake
// CDN API for the tests below, the same pattern newArvanCloudRateLimitProvider
// uses.
func newArvanCloudHealthCheckProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
	t.Helper()

	client, err := arvancloud.New(arvancloud.WithBaseURL(providerSrv.URL))
	if err != nil {
		t.Fatalf("arvancloud.New() error = %v", err)
	}
	provider, err := arvancloud.NewProvider(client)
	if err != nil {
		t.Fatalf("arvancloud.NewProvider() error = %v", err)
	}
	return provider
}

// TestCreateArvanCloudHealthCheckTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip for a TCP check, proving
// origin_type defaults to "pool" and status defaults to true when omitted.
func TestCreateArvanCloudHealthCheckTool(t *testing.T) {
	var path string
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"id":"hc-1","name":"db-check","origin":"pool-1","origin_type":"pool",
			"interval":30000,"threshold":3,"type":"TCP","status":true,"request_config":{"port":5432,"timeout":3000}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudHealthCheckProvider(t, providerSrv)
	tool := createArvanCloudHealthCheckTool(app.NewCreateArvanCloudHealthCheck(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"name":        "db-check",
		"origin":      "pool-1",
		"interval_ms": 30000,
		"threshold":   3,
		"type":        "TCP",
		"tcp_config":  map[string]any{"port": 5432, "timeout_ms": 3000},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/health-checks" {
		t.Errorf("request path = %q, want /domains/example.com/health-checks", path)
	}
	if body["origin_type"] != "pool" {
		t.Errorf("request body[origin_type] = %v, want \"pool\" (the default)", body["origin_type"])
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != true {
		t.Errorf("request body[status] = %v, hasKey=%v, want true (the default)", status, hasStatus)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "hc-1" {
		t.Errorf("result[id] = %v, want %q", out["id"], "hc-1")
	}
	tcpConfig, ok := out["tcp_config"].(map[string]any)
	if !ok || tcpConfig["port"] != 5432 {
		t.Errorf("result[tcp_config] = %#v, want port=5432", out["tcp_config"])
	}
}

// TestCreateArvanCloudHealthCheckToolRejectsBadInterval proves the interval
// validation applies at the tool layer too, not only inside the use case
// tested directly in app_test.
func TestCreateArvanCloudHealthCheckToolRejectsBadInterval(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("provider was called; want the request rejected before it reached the provider")
	}))
	defer providerSrv.Close()

	provider := newArvanCloudHealthCheckProvider(t, providerSrv)
	tool := createArvanCloudHealthCheckTool(app.NewCreateArvanCloudHealthCheck(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"name":        "db-check",
		"origin":      "pool-1",
		"interval_ms": 30, // seconds, not milliseconds — must be rejected
		"threshold":   3,
		"type":        "TCP",
		"tcp_config":  map[string]any{"port": 5432, "timeout_ms": 3000},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Errorf("Handler() error = nil, want a validation error for interval_ms=30")
	}
}

// TestListArvanCloudHealthChecksTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip.
func TestListArvanCloudHealthChecksTool(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"hc-1","name":"db-check","origin":"pool-1","origin_type":"pool",
			"interval":30000,"threshold":3,"type":"TCP","status":true,"request_config":{"port":5432,"timeout":3000}}]}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudHealthCheckProvider(t, providerSrv)
	tool := listArvanCloudHealthChecksTool(app.NewListArvanCloudHealthChecks(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	checks, ok := out["health_checks"].([]map[string]any)
	if !ok || len(checks) != 1 || checks[0]["id"] != "hc-1" {
		t.Errorf("result[health_checks] = %#v, want one check with id=hc-1", out["health_checks"])
	}
}

// TestGetArvanCloudHealthCheckDetailsTool pins the tool -> adapter query
// string and the paginated response's "page" field being surfaced.
func TestGetArvanCloudHealthCheckDetailsTool(t *testing.T) {
	var query string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"date":"2026-08-21T00:00:00Z","zone":"fra","upstream":"1.1.1.1","status":false,"message":"timeout"}],
			"meta":{"current_page":1,"from":1,"last_page":1,"per_page":25,"to":1,"total":1}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudHealthCheckProvider(t, providerSrv)
	tool := getArvanCloudHealthCheckDetailsTool(app.NewGetArvanCloudHealthCheckDetails(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":  "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":   "example.com",
		"name":     "db-check",
		"upstream": "1.1.1.1",
		"type":     "error",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if query != "name=db-check&type=error&upstream=1.1.1.1" {
		t.Errorf("request query = %q, want name=db-check&type=error&upstream=1.1.1.1", query)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	page, ok := out["page"].(map[string]any)
	if !ok || page["total"] != 1 {
		t.Errorf("result[page] = %#v, want total=1", out["page"])
	}
}
