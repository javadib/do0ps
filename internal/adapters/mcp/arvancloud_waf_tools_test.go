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

// newArvanCloudWafProvider wires an arvancloud.Provider onto a fake CDN API
// for the tests below, the same pattern as newArvanCloudFirewallProvider.
func newArvanCloudWafProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestListArvanCloudWafPresetsTool is an end-to-end tool -> use case -> real
// adapter -> fake HTTP server round trip.
func TestListArvanCloudWafPresetsTool(t *testing.T) {
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":{"presets":[{"id":"preset-1","name":"OWASP Basic"}],"packages":[{"id":"pkg-1","name":"core-rules"}]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := listArvanCloudWafPresetsTool(app.NewListArvanCloudWafPresets(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/waf" {
		t.Errorf("request path = %q, want /waf", path)
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	presets, ok := out["presets"].([]map[string]any)
	if !ok || len(presets) != 1 || presets[0]["id"] != "preset-1" {
		t.Errorf("result[presets] = %#v, want a single preset-1 entry", out["presets"])
	}
}

// TestUpdateArvanCloudWafSettingsTool pins the tool -> adapter request body,
// confirming mode is sent as the plain JSON string, and reads the tool's
// own response mapping back.
func TestUpdateArvanCloudWafSettingsTool(t *testing.T) {
	var body []byte
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"data":{"is_enabled":true,"mode":"protect","log_redaction":null,"packages":[]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := updateArvanCloudWafSettingsTool(app.NewUpdateArvanCloudWafSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"mode":    "protect",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPatch || path != "/domains/example.com/waf/settings" {
		t.Errorf("request = %s %s, want PATCH /domains/example.com/waf/settings", method, path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if got, ok := sent["mode"].(string); !ok || got != "protect" {
		t.Errorf(`request body["mode"] = %#v, want the string "protect"`, sent["mode"])
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["mode"] != "protect" {
		t.Errorf("result[mode] = %v, want protect", out["mode"])
	}
}

// TestUpdateArvanCloudWafSettingsToolRejectsFalseAsMode proves the tool
// rejects the literal string "false" as a mode value — the encoding the
// issue text warned about but the spec does not actually use (see
// domain.ArvanCloudWafMode's doc comment) — before the provider is ever
// called.
func TestUpdateArvanCloudWafSettingsToolRejectsFalseAsMode(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := updateArvanCloudWafSettingsTool(app.NewUpdateArvanCloudWafSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"mode":    "false",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want an error rejecting mode=\"false\"")
	}
	if called {
		t.Error("provider was called, want validation to reject the request first")
	}
}

// TestReconfigureArvanCloudWafTool is an end-to-end round trip for the
// preset-apply action.
func TestReconfigureArvanCloudWafTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"message":"OK"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := reconfigureArvanCloudWafTool(app.NewReconfigureArvanCloudWaf(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":   "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":    "example.com",
		"preset_id": "preset-1",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/waf/actions/reconfigure" {
		t.Errorf("request path = %q, want /domains/example.com/waf/actions/reconfigure", path)
	}
	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["preset_id"] != "preset-1" {
		t.Errorf(`request body["preset_id"] = %v, want "preset-1"`, sent["preset_id"])
	}

	out, ok := result.(map[string]any)
	if !ok || out["reconfigured"] != true {
		t.Errorf("result = %#v, want reconfigured=true", result)
	}
}

// TestCreateArvanCloudWafRuleTool is an end-to-end tool -> use case -> real
// adapter -> fake HTTP server round trip for a WAF custom rule.
func TestCreateArvanCloudWafRuleTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"rule-1","url_pattern":"/wp-admin/**","sources":["203.0.113.0/24"],"action":"passthrough","is_enabled":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := createArvanCloudWafRuleTool(app.NewCreateArvanCloudWafRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"url_pattern": "/wp-admin/**",
		"sources":     []string{"203.0.113.0/24"},
		"action":      "passthrough",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/waf/rules" {
		t.Errorf("request path = %q, want /domains/example.com/waf/rules", path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["url_pattern"] != "/wp-admin/**" || sent["action"] != "passthrough" {
		t.Errorf("request body = %+v, want url_pattern/action set", sent)
	}
	if sent["is_enabled"] != true {
		t.Errorf(`request body["is_enabled"] = %v, want true (the tool's default)`, sent["is_enabled"])
	}

	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want a map", result)
	}
	if out["id"] != "rule-1" {
		t.Errorf("result[id] = %v, want rule-1", out["id"])
	}
}

// TestCreateArvanCloudWafRuleToolRejectsFirewallAction proves the WAF
// custom-rule tool rejects an action from the CDN edge Firewall's own enum
// (e.g. "allow") before the provider is ever called — the two capabilities'
// action enums must not be conflated at the tool boundary either (issue
// #66's acceptance criteria, mirroring
// TestCreateArvanCloudFirewallRuleToolRejectsDropAction).
func TestCreateArvanCloudWafRuleToolRejectsFirewallAction(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := createArvanCloudWafRuleTool(app.NewCreateArvanCloudWafRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"url_pattern": "/x/**",
		"action":      "allow",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want an error rejecting a Firewall-only action")
	}
	if called {
		t.Error("provider was called, want validation to reject the request first")
	}
}

// TestInstallArvanCloudWafPackageTool is an end-to-end round trip for
// installing a global package onto a domain.
func TestInstallArvanCloudWafPackageTool(t *testing.T) {
	var body []byte
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"data":{"id":"pkg-1","name":"core-rules","is_enabled":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := installArvanCloudWafPackageTool(app.NewInstallArvanCloudWafPackage(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":    "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":     "example.com",
		"package_id": "pkg-1",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodPost || path != "/domains/example.com/waf/packages" {
		t.Errorf("request = %s %s, want POST /domains/example.com/waf/packages", method, path)
	}
	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["id"] != "pkg-1" {
		t.Errorf(`request body["id"] = %v, want "pkg-1"`, sent["id"])
	}

	out, ok := result.(map[string]any)
	if !ok || out["id"] != "pkg-1" {
		t.Errorf("result = %#v, want id=pkg-1", result)
	}
}

// TestUninstallArvanCloudWafPackageTool is an end-to-end round trip for
// removing an installed package.
func TestUninstallArvanCloudWafPackageTool(t *testing.T) {
	var path, method string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		method = r.Method
		_, _ = w.Write([]byte(`{"message":"OK"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudWafProvider(t, providerSrv)
	tool := uninstallArvanCloudWafPackageTool(app.NewUninstallArvanCloudWafPackage(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"id":      "pkg-1",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if method != http.MethodDelete || path != "/domains/example.com/waf/packages/pkg-1" {
		t.Errorf("request = %s %s, want DELETE /domains/example.com/waf/packages/pkg-1", method, path)
	}

	out, ok := result.(map[string]any)
	if !ok || out["uninstalled"] != true {
		t.Errorf("result = %#v, want uninstalled=true", result)
	}
}
