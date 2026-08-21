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

// newArvanCloudDdosProvider wires an arvancloud.Provider onto a fake CDN API
// for the tests below, the same pattern as newArvanCloudWafProvider.
func newArvanCloudDdosProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestGetArvanCloudDdosSettingsTool is an end-to-end tool -> use case -> real
// adapter -> fake HTTP server round trip.
func TestGetArvanCloudDdosSettingsTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"is_enabled":true,"protection_mode":"cookie","ttl":3600,"https_only":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := getArvanCloudDdosSettingsTool(app.NewGetArvanCloudDdosSettings(inlineQueue{}, provider))

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
	if path != "/domains/example.com/ddos/settings" {
		t.Errorf("request path = %q, want /domains/example.com/ddos/settings", path)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["protection_mode"] != "cookie" {
		t.Errorf("result[protection_mode] = %v, want cookie", out["protection_mode"])
	}
}

// TestUpdateArvanCloudDdosSettingsTool pins the tool -> adapter request body,
// confirming protection_mode/captcha fields are sent as given and
// captcha_secret_key maps onto the settings' secret_key field (distinct from
// this tool's own top-level secret_key credential parameter — see
// arvancloud_ddos_tools.go's updateArvanCloudDdosSettingsTool).
func TestUpdateArvanCloudDdosSettingsTool(t *testing.T) {
	var body []byte
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"data":{"is_enabled":true,"protection_mode":"captcha","captcha_service":"hcaptcha",
			"site_key":"site-xyz","secret_key":"secret-xyz","ttl":300,"https_only":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := updateArvanCloudDdosSettingsTool(app.NewUpdateArvanCloudDdosSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":            "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":             "example.com",
		"protection_mode":    "captcha",
		"captcha_service":    "hcaptcha",
		"captcha_site_key":   "site-xyz",
		"captcha_secret_key": "secret-xyz",
		"ttl":                300,
		"https_only":         true,
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPatch || path != "/domains/example.com/ddos/settings" {
		t.Errorf("request = %s %s, want PATCH /domains/example.com/ddos/settings", method, path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["protection_mode"] != "captcha" || sent["captcha_service"] != "hcaptcha" {
		t.Errorf("request body = %+v, want protection_mode/captcha_service sent", sent)
	}
	if sent["site_key"] != "site-xyz" || sent["secret_key"] != "secret-xyz" {
		t.Errorf(`request body = %+v, want the tool's captcha_site_key/captcha_secret_key mapped onto site_key/secret_key`, sent)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["protection_mode"] != "captcha" || out["captcha_service"] != "hcaptcha" {
		t.Errorf("result = %#v, want protection_mode=captcha, captcha_service=hcaptcha", out)
	}
}

// TestUpdateArvanCloudDdosSettingsToolRejectsFalseAsProtectionMode proves the
// tool rejects the literal string "false" as a protection_mode value — the
// encoding issue #67 warned might apply but which the spec does not actually
// use (see domain.ArvanCloudDdosProtectionMode's doc comment) — before the
// provider is ever called.
func TestUpdateArvanCloudDdosSettingsToolRejectsFalseAsProtectionMode(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := updateArvanCloudDdosSettingsTool(app.NewUpdateArvanCloudDdosSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":         "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":          "example.com",
		"protection_mode": "false",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want an error rejecting protection_mode=\"false\"")
	}
	if called {
		t.Error("provider was called, want validation to reject the request first")
	}
}

// TestUpdateArvanCloudDdosSettingsToolRequiresCaptchaServiceForCaptchaMode
// proves the tool rejects protection_mode="captcha" without a
// captcha_service, before the provider is ever called.
func TestUpdateArvanCloudDdosSettingsToolRequiresCaptchaServiceForCaptchaMode(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := updateArvanCloudDdosSettingsTool(app.NewUpdateArvanCloudDdosSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":         "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":          "example.com",
		"protection_mode": "captcha",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want an error requiring captcha_service")
	}
	if called {
		t.Error("provider was called, want validation to reject the request first")
	}
}

// TestCreateArvanCloudDdosRuleTool is an end-to-end tool -> use case -> real
// adapter -> fake HTTP server round trip for a DDoS rule.
func TestCreateArvanCloudDdosRuleTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"id":"rule-1","url_pattern":"/wp-admin/**","sources":["203.0.113.0/24"],"action":"protect","is_enabled":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := createArvanCloudDdosRuleTool(app.NewCreateArvanCloudDdosRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"url_pattern": "/wp-admin/**",
		"sources":     []string{"203.0.113.0/24"},
		"action":      "protect",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/ddos/rules" {
		t.Errorf("request path = %q, want /domains/example.com/ddos/rules", path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["url_pattern"] != "/wp-admin/**" || sent["action"] != "protect" {
		t.Errorf("request body = %+v, want url_pattern/action sent", sent)
	}

	out, ok := result.(map[string]any)
	if !ok || out["id"] != "rule-1" {
		t.Errorf("result = %#v, want id=rule-1", result)
	}
}

// TestDeleteArvanCloudDdosRuleTool is an end-to-end round trip for the
// delete tool.
func TestDeleteArvanCloudDdosRuleTool(t *testing.T) {
	var method, path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"message":"Deleted successfully"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := deleteArvanCloudDdosRuleTool(app.NewDeleteArvanCloudDdosRule(inlineQueue{}, provider))

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
	if method != http.MethodDelete || path != "/domains/example.com/ddos/rules/rule-1" {
		t.Errorf("request = %s %s, want DELETE /domains/example.com/ddos/rules/rule-1", method, path)
	}
	out, ok := result.(map[string]any)
	if !ok || out["deleted"] != true {
		t.Errorf("result = %#v, want deleted=true", result)
	}
}

// TestReprioritizeArvanCloudDdosRulesTool is an end-to-end round trip for the
// reprioritize tool.
func TestReprioritizeArvanCloudDdosRulesTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"message":"OK"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudDdosProvider(t, providerSrv)
	tool := reprioritizeArvanCloudDdosRulesTool(app.NewReprioritizeArvanCloudDdosRules(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":        "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":         "example.com",
		"rule_id":        "rule-1",
		"before_rule_id": "rule-2",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/ddos/actions/reprioritize" {
		t.Errorf("request path = %q, want /domains/example.com/ddos/actions/reprioritize", path)
	}
	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["rule_id"] != "rule-1" || sent["before_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id/before_rule_id set", sent)
	}

	out, ok := result.(map[string]any)
	if !ok || out["reprioritized"] != true {
		t.Errorf("result = %#v, want reprioritized=true", result)
	}
}
