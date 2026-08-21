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

// newArvanCloudObservabilityProvider wires an arvancloud.Provider onto a
// fake CDN API for the tests below, the same pattern
// newArvanCloudHealthCheckProvider uses.
func newArvanCloudObservabilityProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestCreateArvanCloudLogForwarderTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip for an S3 connection type,
// proving the flat "settings" object is routed to the right tagged branch
// and status defaults to true when omitted.
func TestCreateArvanCloudLogForwarderTool(t *testing.T) {
	var path, method string
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{
			"id":"lf-1","name":"s3 forwarder","description":"ships access logs to s3","type":"access",
			"connection_type":"arvan_s3","status":true,
			"settings":{"s3_endpoint":"s3.example.com","access_key":"AKIA","secret_key":"shh","bucket_name":"logs"}
		}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := createArvanCloudLogForwarderTool(app.NewCreateArvanCloudLogForwarder(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":         "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":          "example.com",
		"name":            "s3 forwarder",
		"description":     "ships access logs to s3",
		"type":            "access",
		"connection_type": "arvan_s3",
		"settings": map[string]any{
			"s3_endpoint": "s3.example.com", "access_key": "AKIA", "secret_key": "shh", "bucket_name": "logs",
		},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPost || path != "/domains/example.com/log-forwarders" {
		t.Fatalf("request = %s %s, want POST /domains/example.com/log-forwarders", method, path)
	}
	status, hasStatus := body["status"]
	if !hasStatus || status != true {
		t.Errorf("request body[status] = %v, hasKey=%v, want true (the default)", status, hasStatus)
	}
	settings, ok := body["settings"].(map[string]any)
	if !ok || settings["access_key"] != "AKIA" || settings["secret_key"] != "shh" {
		t.Errorf("request body[settings] = %#v, want the S3 fields sent", body["settings"])
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "lf-1" || out["connection_type"] != "arvan_s3" {
		t.Errorf("result[id]/[connection_type] = %v/%v, want lf-1/arvan_s3", out["id"], out["connection_type"])
	}
	resultSettings, ok := out["settings"].(map[string]any)
	if !ok || resultSettings["bucket_name"] != "logs" {
		t.Errorf("result[settings] = %#v, want bucket_name=logs", out["settings"])
	}
}

// TestCreateArvanCloudLogForwarderToolRejectsMissingSettings proves the use
// case's validation reaches the tool layer as an error, not a panic.
func TestCreateArvanCloudLogForwarderToolRejectsMissingSettings(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("provider should not be called when validation fails")
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := createArvanCloudLogForwarderTool(app.NewCreateArvanCloudLogForwarder(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":         "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":          "example.com",
		"name":            "s3 forwarder",
		"description":     "ships access logs to s3",
		"type":            "access",
		"connection_type": "arvan_s3",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want a validation error for missing settings")
	}
}

// TestListArvanCloudLogForwardersTool proves list decoding, including page
// metadata.
func TestListArvanCloudLogForwardersTool(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"id":"lf-1","type":"access","connection_type":"arvan_s3","name":"one","description":"d","status":true}
		],"meta":{"current_page":1,"last_page":1,"per_page":20,"total":1}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := listArvanCloudLogForwardersTool(app.NewListArvanCloudLogForwarders(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com"})
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
	forwarders, ok := out["log_forwarders"].([]map[string]any)
	if !ok || len(forwarders) != 1 || forwarders[0]["id"] != "lf-1" {
		t.Errorf("result[log_forwarders] = %#v, want one entry with id lf-1", out["log_forwarders"])
	}
	page, ok := out["page"].(map[string]any)
	if !ok || page["total"] != 1 {
		t.Errorf("result[page] = %#v, want total=1", out["page"])
	}
}

// TestSetArvanCloudLogForwarderStatusTool pins the PATCH .../status
// round trip.
func TestSetArvanCloudLogForwarderStatusTool(t *testing.T) {
	var path, method string
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"id":"lf-1","name":"one","description":"d","type":"access","connection_type":"arvan_s3","status":false,"settings":{}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := setArvanCloudLogForwarderStatusTool(app.NewSetArvanCloudLogForwarderStatus(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com", "id": "lf-1", "status": false,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPatch || path != "/domains/example.com/log-forwarders/lf-1/status" {
		t.Fatalf("request = %s %s, want PATCH /domains/example.com/log-forwarders/lf-1/status", method, path)
	}
	if body["status"] != false {
		t.Errorf("request body = %+v, want status=false", body)
	}
	out, ok := result.(map[string]any)
	if !ok || out["status"] != false {
		t.Errorf("result = %#v, want status=false", result)
	}
}

// TestCreateArvanCloudMetricExporterTool is an end-to-end round trip,
// scoped to a domain even though listing is account-wide.
func TestCreateArvanCloudMetricExporterTool(t *testing.T) {
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		_, _ = w.Write([]byte(`{"data":{"id":"me-1","name":"exporter one","type":"access","interval":"30s","url":"https://metrics.example/one","status":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := createArvanCloudMetricExporterTool(app.NewCreateArvanCloudMetricExporter(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com",
		"name": "exporter one", "type": "access", "interval": "30s",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPost || path != "/domains/example.com/metric-exporters" {
		t.Fatalf("request = %s %s, want POST /domains/example.com/metric-exporters", method, path)
	}
	out, ok := result.(map[string]any)
	if !ok || out["id"] != "me-1" || out["url"] != "https://metrics.example/one" {
		t.Errorf("result = %#v, want id=me-1 and the parsed url", result)
	}
}

// TestListArvanCloudMetricExportersTool proves the account-wide list
// (no /domains/{domain} prefix) round trip and per-item "domain" field.
func TestListArvanCloudMetricExportersTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[
			{"id":"me-1","type":"access","interval":"30s","name":"one","domain":"example.com","status":true}
		],"meta":{"current_page":1,"last_page":1,"per_page":20,"total":1}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := listArvanCloudMetricExportersTool(app.NewListArvanCloudMetricExporters(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/metric-exporters" {
		t.Errorf("request path = %q, want /metric-exporters (account-wide)", path)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	exporters, ok := out["metric_exporters"].([]map[string]any)
	if !ok || len(exporters) != 1 || exporters[0]["domain"] != "example.com" {
		t.Errorf("result[metric_exporters] = %#v, want one entry with domain=example.com", out["metric_exporters"])
	}
}

// TestListArvanCloudMetricExporterTypesTool pins GET /metric-exporters/metrics.
func TestListArvanCloudMetricExporterTypesTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"metric":"access","items":[{"name":"requests_total","description":"total requests"}]}],"message":"ok"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudObservabilityProvider(t, providerSrv)
	tool := listArvanCloudMetricExporterTypesTool(app.NewListArvanCloudMetricExporterTypes(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/metric-exporters/metrics" {
		t.Errorf("request path = %q, want /metric-exporters/metrics", path)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	groups, ok := out["metric_groups"].([]map[string]any)
	if !ok || len(groups) != 1 || groups[0]["metric"] != "access" {
		t.Errorf("result[metric_groups] = %#v, want one group named access", out["metric_groups"])
	}
	if out["message"] != "ok" {
		t.Errorf("result[message] = %v, want %q", out["message"], "ok")
	}
}
