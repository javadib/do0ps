package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/arvancloud"
	"github.com/javadib/do0ps/internal/core/app"
)

// newArvanCloudReportsProvider wires an arvancloud.Provider onto a fake CDN
// API for the tests below, the same pattern newArvanCloudHealthCheckProvider
// uses.
func newArvanCloudReportsProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestGetArvanCloudTrafficReportTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip, proving the time-range query
// parameters reach the request and the chart/statistics fields reach the
// tool result.
func TestGetArvanCloudTrafficReportTool(t *testing.T) {
	var path, query string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":{"statistics":{"traffics":{"saved":1,"bypass":0,"top":"t","total":10},
			"requests":{"saved":2,"bypass":0,"top":"t2","total":20}},
			"charts":{"requests":{"title":"reports.requests","categories":["c1"],"series":[]},
			"traffics":{"title":"reports.traffics","categories":["c1"],"series":[]}}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudReportsProvider(t, providerSrv)
	tool := getArvanCloudTrafficReportTool(app.NewGetArvanCloudTrafficReport(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"period":  "24h",
		"since":   "2026-08-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/reports/traffics" {
		t.Errorf("request path = %q, want /domains/example.com/reports/traffics", path)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("parsing request query %q: %v", query, err)
	}
	if values.Get("period") != "24h" || values.Get("since") != "2026-08-01T00:00:00Z" {
		t.Errorf("request query = %q, want period=24h and since=2026-08-01T00:00:00Z", query)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	stats, ok := out["traffic_statistics"].(map[string]any)
	if !ok || stats["total"] != int64(10) {
		t.Errorf("result[traffic_statistics] = %#v, want total=10", out["traffic_statistics"])
	}
}

// TestGetArvanCloudHighRequestIPsTool proves the paginated
// {"data":...,"meta":...} envelope reaches the tool result as "ips"/"page".
func TestGetArvanCloudHighRequestIPsTool(t *testing.T) {
	var query string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"ip":"1.1.1.1","request_count":50}],
			"meta":{"current_page":1,"from":1,"last_page":2,"per_page":10,"to":10,"total":15}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudReportsProvider(t, providerSrv)
	tool := getArvanCloudHighRequestIPsTool(app.NewListArvanCloudHighRequestIPs(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":  "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":   "example.com",
		"per_page": 10,
		"page":     1,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("parsing request query %q: %v", query, err)
	}
	if values.Get("per_page") != "10" || values.Get("page") != "1" {
		t.Errorf("request query = %q, want per_page=10 and page=1", query)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	ips, ok := out["ips"].([]map[string]any)
	if !ok || len(ips) != 1 || ips[0]["request_count"] != int64(50) {
		t.Errorf("result[ips] = %#v, want one entry with request_count=50", out["ips"])
	}
	page, ok := out["page"].(map[string]any)
	if !ok || page["total"] != 15 {
		t.Errorf("result[page] = %#v, want total=15", out["page"])
	}
}

// TestGetArvanCloudTransportLayerProxyTrafficTool proves
// transport_layer_proxy_id reaches the request path as a caller-supplied
// opaque ID (ports.ArvanCloudProvider's own doc comment).
func TestGetArvanCloudTransportLayerProxyTrafficTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"charts":{"traffics":{"title":"reports.traffics","categories":[],"series":[]}}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudReportsProvider(t, providerSrv)
	tool := getArvanCloudTransportLayerProxyTrafficTool(app.NewGetArvanCloudTransportLayerProxyTraffic(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":                  "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":                   "example.com",
		"transport_layer_proxy_id": "proxy-abc",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if want := "/domains/example.com/reports/transport-layer-proxies/proxy-abc/traffics"; path != want {
		t.Errorf("request path = %q, want %q", path, want)
	}
}

// TestDownloadArvanCloudDomainsReportTool proves this tool is NOT scoped to
// a single domain (no "domain" argument) and passes the CSV body through
// unparsed.
func TestDownloadArvanCloudDomainsReportTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte("domain,requests\nexample.com,42\n"))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudReportsProvider(t, providerSrv)
	tool := downloadArvanCloudDomainsReportTool(app.NewDownloadArvanCloudDomainsReport(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/reports/download" {
		t.Errorf("request path = %q, want /domains/reports/download", path)
	}
	out, ok := result.(map[string]any)
	if !ok || out["csv"] != "domain,requests\nexample.com,42\n" {
		t.Errorf("result = %#v, want the raw csv body", result)
	}
}

// TestGetArvanCloudAggregatedReportFiltersTool proves this Aggregated
// Reports tool is account-wide (no "domain" argument) and its "domains"
// filter reaches the request query.
func TestGetArvanCloudAggregatedReportFiltersTool(t *testing.T) {
	var path, query string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":{"pops":[{"pop":"thr-mci","label":"Tehran - MCI"}],"asns":[6439]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudReportsProvider(t, providerSrv)
	tool := getArvanCloudAggregatedReportFiltersTool(app.NewGetArvanCloudAggregatedReportFilters(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domains": "example1.com,example2.com",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/reports/aggregated/filters" {
		t.Errorf("request path = %q, want /reports/aggregated/filters", path)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("parsing request query %q: %v", query, err)
	}
	if values.Get("domains") != "example1.com,example2.com" {
		t.Errorf("request query = %q, want domains=example1.com,example2.com", query)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	pops, ok := out["pops"].([]map[string]any)
	if !ok || len(pops) != 1 || pops[0]["pop"] != "thr-mci" {
		t.Errorf("result[pops] = %#v, want one entry with pop=thr-mci", out["pops"])
	}
}
