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

// newArvanCloudRulesProvider wires an arvancloud.Provider onto a fake CDN
// API for the tests below, the same pattern newArvanCloudHealthCheckProvider
// uses.
func newArvanCloudRulesProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestCreateArvanCloudPageRuleTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip, proving the grouped input
// schema (matching/caching/security/routing/headers/other) decodes and
// reaches the provider correctly, one field from every group.
func TestCreateArvanCloudPageRuleTool(t *testing.T) {
	var path string
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"id":"pr-1","url":"/api/*","status":true,"cache_level":"uri",
			"waf_status":true,"load_balancer":"lb-1","cors_header":"*","acceleration":{"status":"on"}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := createArvanCloudPageRuleTool(app.NewCreateArvanCloudPageRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":  "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":   "example.com",
		"matching": map[string]any{"url": "/api/*", "status": true},
		"caching":  map[string]any{"cache_level": "uri", "cache_200": "30m"},
		"security": map[string]any{"waf_status": true},
		"routing":  map[string]any{"load_balancer": "lb-1"},
		"headers":  map[string]any{"cors_header": "*"},
		"other":    map[string]any{"acceleration": map[string]any{"status": "on"}},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/page-rules" {
		t.Errorf("request path = %q, want /domains/example.com/page-rules", path)
	}
	if body["url"] != "/api/*" || body["cache_level"] != "uri" || body["waf_status"] != true || body["load_balancer"] != "lb-1" || body["cors_header"] != "*" {
		t.Errorf("request body = %#v, missing an expected field from one of the groups", body)
	}
	acceleration, ok := body["acceleration"].(map[string]any)
	if !ok || acceleration["status"] != "on" {
		t.Errorf("request body[acceleration] = %#v, want status=on", body["acceleration"])
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "pr-1" {
		t.Errorf("result[id] = %v, want pr-1", out["id"])
	}
}

// TestCreateArvanCloudPageRuleToolRejectsMissingURL proves the matching.url
// validation applies at the tool layer too.
func TestCreateArvanCloudPageRuleToolRejectsMissingURL(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("provider was called; want the request rejected before it reached the provider")
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := createArvanCloudPageRuleTool(app.NewCreateArvanCloudPageRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":  "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":   "example.com",
		"matching": map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Errorf("Handler() error = nil, want a validation error for a missing matching.url")
	}
}

// TestListArvanCloudPageRulesTool is an end-to-end round trip.
func TestListArvanCloudPageRulesTool(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"pr-1","url":"/api/*","status":true}],
			"meta":{"current_page":1,"from":1,"last_page":1,"per_page":25,"to":1,"total":1}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := listArvanCloudPageRulesTool(app.NewListArvanCloudPageRules(inlineQueue{}, provider))

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
	rules, ok := out["page_rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["id"] != "pr-1" {
		t.Errorf("result[page_rules] = %#v, want one rule with id=pr-1", out["page_rules"])
	}
}

// TestSetArvanCloudPageRuleStatusTool is an end-to-end round trip.
func TestSetArvanCloudPageRuleStatusTool(t *testing.T) {
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := setArvanCloudPageRuleStatusTool(app.NewSetArvanCloudPageRuleStatus(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com", "id": "pr-1", "status": false,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if body["status"] != false {
		t.Errorf("request body[status] = %v, want false", body["status"])
	}
	out, ok := result.(map[string]any)
	if !ok || out["status"] != false {
		t.Errorf("result = %#v, want status=false", result)
	}
}

// TestCreateArvanCloudResponseTransformTool is an end-to-end round trip.
func TestCreateArvanCloudResponseTransformTool(t *testing.T) {
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"id":"rt-1","name":"cors-preset","transforms":[{"condition":"true",
			"actions":[{"type":"add_header","mode":"set","key":"Access-Control-Allow-Origin","value":"to_string(\"*\")"}]}]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := createArvanCloudResponseTransformTool(app.NewCreateArvanCloudResponseTransform(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com", "name": "cors-preset",
		"transforms": []map[string]any{{
			"condition": "true",
			"actions":   []map[string]any{{"key": "Access-Control-Allow-Origin", "value": `to_string("*")`}},
		}},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if body["name"] != "cors-preset" {
		t.Errorf("request body[name] = %v, want cors-preset", body["name"])
	}
	out, ok := result.(map[string]any)
	if !ok || out["id"] != "rt-1" {
		t.Errorf("result = %#v, want id=rt-1", result)
	}
}

// TestUpdateArvanCloudWWWRedirectTool is an end-to-end round trip.
func TestUpdateArvanCloudWWWRedirectTool(t *testing.T) {
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := updateArvanCloudWWWRedirectTool(app.NewUpdateArvanCloudWWWRedirect(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com", "mode": "www"})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if body["f_redirect_to_www"] != "www" {
		t.Errorf("request body[f_redirect_to_www] = %v, want www", body["f_redirect_to_www"])
	}
}

// TestAddArvanCloudHostHeaderWhitelistEntryTool is an end-to-end round trip.
func TestAddArvanCloudHostHeaderWhitelistEntryTool(t *testing.T) {
	var body map[string]any
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":{"target_accounts":["acct-1"],"globally_whitelisted":false}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRulesProvider(t, providerSrv)
	tool := addArvanCloudHostHeaderWhitelistEntryTool(app.NewAddArvanCloudHostHeaderWhitelistEntry(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d", "domain": "example.com", "target_account": "acct-1",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if body["target_account"] != "acct-1" {
		t.Errorf("request body[target_account] = %v, want acct-1", body["target_account"])
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	accounts, ok := out["target_accounts"].([]string)
	if !ok || len(accounts) != 1 || accounts[0] != "acct-1" {
		t.Errorf("result[target_accounts] = %#v, want [acct-1]", out["target_accounts"])
	}
}
