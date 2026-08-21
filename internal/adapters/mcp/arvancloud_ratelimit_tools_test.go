package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javadib/do0ps/internal/adapters/providers/arvancloud"
	"github.com/javadib/do0ps/internal/core/app"
)

// newArvanCloudRateLimitProvider wires an arvancloud.Provider onto a fake CDN
// API for the tests below, the same pattern as newArvanCloudDdosProvider.
func newArvanCloudRateLimitProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestGetArvanCloudRateLimitSettingsTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip.
func TestGetArvanCloudRateLimitSettingsTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"ddos_detection":true,"exclude_sources":["203.0.113.0/24"]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRateLimitProvider(t, providerSrv)
	tool := getArvanCloudRateLimitSettingsTool(app.NewGetArvanCloudRateLimitSettings(inlineQueue{}, provider))

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
	if path != "/domains/example.com/rate-limit/settings" {
		t.Errorf("request path = %q, want /domains/example.com/rate-limit/settings", path)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["ddos_detection"] != true {
		t.Errorf("result[ddos_detection] = %v, want true", out["ddos_detection"])
	}
}

// TestUpdateArvanCloudRateLimitSettingsTool pins the tool -> adapter request
// body.
func TestUpdateArvanCloudRateLimitSettingsTool(t *testing.T) {
	var body []byte
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"data":{"ddos_detection":true,"exclude_sources":["203.0.113.0/24"]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRateLimitProvider(t, providerSrv)
	tool := updateArvanCloudRateLimitSettingsTool(app.NewUpdateArvanCloudRateLimitSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":         "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":          "example.com",
		"ddos_detection":  true,
		"exclude_sources": []string{"203.0.113.0/24"},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPatch || path != "/domains/example.com/rate-limit/settings" {
		t.Errorf("request = %s %s, want PATCH /domains/example.com/rate-limit/settings", method, path)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if decoded["ddos_detection"] != true {
		t.Errorf("request body[ddos_detection] = %v, want true", decoded["ddos_detection"])
	}
}

// TestCreateArvanCloudRateLimitRuleTool pins the tool -> adapter request
// body, including units (rate/burst are request counts, time_duration/
// block_duration are seconds — AGENTS.md 5) reaching the provider as plain
// integers.
func TestCreateArvanCloudRateLimitRuleTool(t *testing.T) {
	var body []byte
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"data":{"id":"rule-1","url_pattern":"/login","action":"challenge","is_enabled":true,
			"rate":5,"time_duration":60,"action_details":{"mode":3,"ttl":600,"https_only":true}}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRateLimitProvider(t, providerSrv)
	tool := createArvanCloudRateLimitRuleTool(app.NewCreateArvanCloudRateLimitRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":       "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":        "example.com",
		"url_pattern":   "/login",
		"action":        "challenge",
		"rate":          5,
		"time_duration": 60,
		"action_details": map[string]any{
			"mode": 3, "ttl": 600, "https_only": true,
		},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPost || path != "/domains/example.com/rate-limit/rules" {
		t.Errorf("request = %s %s, want POST /domains/example.com/rate-limit/rules", method, path)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if decoded["rate"] != float64(5) || decoded["time_duration"] != float64(60) {
		t.Errorf("request body = %+v, want rate/time_duration sent as given", decoded)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "rule-1" {
		t.Errorf("result[id] = %v, want rule-1", out["id"])
	}
}

// TestCreateArvanCloudRateLimitRuleToolRejectsNonPositiveRate proves the
// tool surfaces the use case's client-side rejection (issue #68's
// acceptance criteria) rather than ever reaching the provider.
func TestCreateArvanCloudRateLimitRuleToolRejectsNonPositiveRate(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRateLimitProvider(t, providerSrv)
	tool := createArvanCloudRateLimitRuleTool(app.NewCreateArvanCloudRateLimitRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":       "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":        "example.com",
		"url_pattern":   "/login",
		"rate":          0,
		"time_duration": 60,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want a validation error for rate=0")
	}
	if called {
		t.Error("provider was called, want validation to reject the request first")
	}
}

// TestDeleteArvanCloudRateLimitRuleTool pins the tool -> adapter request
// shape.
func TestDeleteArvanCloudRateLimitRuleTool(t *testing.T) {
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"message":"Deleted successfully"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRateLimitProvider(t, providerSrv)
	tool := deleteArvanCloudRateLimitRuleTool(app.NewDeleteArvanCloudRateLimitRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"id":      "rule-1",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodDelete || path != "/domains/example.com/rate-limit/rules/rule-1" {
		t.Errorf("request = %s %s, want DELETE /domains/example.com/rate-limit/rules/rule-1", method, path)
	}

	out, ok := result.(map[string]any)
	if !ok || out["deleted"] != true {
		t.Errorf("result = %#v, want deleted:true", result)
	}
}

// TestReprioritizeArvanCloudRateLimitRulesTool pins the tool -> adapter
// request body.
func TestReprioritizeArvanCloudRateLimitRulesTool(t *testing.T) {
	var body []byte
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"message":"OK"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudRateLimitProvider(t, providerSrv)
	tool := reprioritizeArvanCloudRateLimitRulesTool(app.NewReprioritizeArvanCloudRateLimitRules(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":       "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":        "example.com",
		"rule_id":       "rule-1",
		"after_rule_id": "rule-2",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPost || path != "/domains/example.com/rate-limit/actions/reprioritize" {
		t.Errorf("request = %s %s, want POST /domains/example.com/rate-limit/actions/reprioritize", method, path)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if decoded["rule_id"] != "rule-1" || decoded["after_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id/after_rule_id set", decoded)
	}
}
