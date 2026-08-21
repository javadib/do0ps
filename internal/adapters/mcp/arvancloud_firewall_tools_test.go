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

// newArvanCloudFirewallProvider wires an arvancloud.Provider onto a fake CDN
// API for the tests below, the same pattern as
// newArvanCloudListsProvider/newArvanCloudDomainProvider.
func newArvanCloudFirewallProvider(t *testing.T, providerSrv *httptest.Server) *arvancloud.Provider {
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

// TestCreateArvanCloudFirewallRuleTool is an end-to-end tool -> use case ->
// real adapter -> fake HTTP server round trip: the create_arvancloud_
// firewall_rule tool sends the request the CDN API expects and returns the
// parsed result.
func TestCreateArvanCloudFirewallRuleTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"rule-1","name":"block-ir","filter_expr":"ip.geoip.country in {\"IR\"}","action":"deny","priority":1,"is_enabled":true}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := createArvanCloudFirewallRuleTool(app.NewCreateArvanCloudFirewallRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"name":        "block-ir",
		"filter_expr": `ip.geoip.country in {"IR"}`,
		"action":      "deny",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/firewall/rules" {
		t.Errorf("request path = %q, want /domains/example.com/firewall/rules", path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["name"] != "block-ir" || sent["action"] != "deny" {
		t.Errorf("request body = %+v, want name=block-ir action=deny", sent)
	}
	// is_enabled defaults to true when the caller omits it.
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

// TestCreateArvanCloudFirewallRuleToolRejectsDropAction proves "drop" — a
// valid firewall settings default_action but not a valid per-rule action —
// is rejected before the provider is ever called (issue #65's acceptance
// criteria, exercised at the tool boundary).
func TestCreateArvanCloudFirewallRuleToolRejectsDropAction(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := createArvanCloudFirewallRuleTool(app.NewCreateArvanCloudFirewallRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":     "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":      "example.com",
		"name":        "drop-everything",
		"filter_expr": "true",
		"action":      "drop",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal(`Handler() error = nil, want a validation error ("drop" is not a valid per-rule action)`)
	}
	if called {
		t.Error("provider was called despite an invalid per-rule action")
	}
}

// TestUpdateArvanCloudFirewallSettingsToolAcceptsDropDefaultAction proves
// the settings tool accepts "drop" as a default_action — the same value
// create_arvancloud_firewall_rule rejects as an action — confirming the two
// action enums validate independently at the tool boundary too.
func TestUpdateArvanCloudFirewallSettingsToolAcceptsDropDefaultAction(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"is_enabled":true,"default_action":"drop","verify_sni":true,"skip_global_whitelist":false,"skip_global_firewall":false}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := updateArvanCloudFirewallSettingsTool(app.NewUpdateArvanCloudFirewallSettings(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":        "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":         "example.com",
		"default_action": "drop",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf(`Handler() error = %v, want nil ("drop" is a valid default_action)`, err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["default_action"] != "drop" {
		t.Errorf(`request body["default_action"] = %v, want "drop"`, sent["default_action"])
	}

	out, ok := result.(map[string]any)
	if !ok || out["default_action"] != "drop" {
		t.Errorf("result = %#v, want default_action=drop", result)
	}
}

// TestDeleteArvanCloudFirewallRuleToolTolerantOfNotFound proves the tool
// reports a missing rule as already deleted rather than an error.
func TestDeleteArvanCloudFirewallRuleToolTolerantOfNotFound(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := deleteArvanCloudFirewallRuleTool(app.NewDeleteArvanCloudFirewallRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":  "example.com",
		"id":      "missing",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v, want nil (already-absent rule tolerated)", err)
	}
	out, ok := result.(map[string]any)
	if !ok || out["deleted"] != true {
		t.Errorf("result = %#v, want deleted=true", result)
	}
}

// TestCreateArvanCloudAccountFirewallRuleToolRejectsEmptyDomainIDsForInclude
// is issue #65's headline acceptance criterion, exercised at the tool
// boundary: domain_selection_type="include" with domain_ids omitted must be
// rejected before the provider is ever called.
func TestCreateArvanCloudAccountFirewallRuleToolRejectsEmptyDomainIDsForInclude(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := createArvanCloudAccountFirewallRuleTool(app.NewCreateArvanCloudAccountFirewallRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":               "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"name":                  "block-scanners",
		"filter_expr":           "true",
		"action":                "deny",
		"domain_selection_type": "include",
		// domain_ids deliberately omitted
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want a validation error for empty domain_ids under domain_selection_type=include")
	}
	if called {
		t.Error("provider was called despite empty domain_ids under domain_selection_type=include")
	}
}

// TestCreateArvanCloudAccountFirewallRuleTool is an end-to-end round trip
// pinning the domain_selection_type/domain_ids request shape for a valid
// "include" rule.
func TestCreateArvanCloudAccountFirewallRuleTool(t *testing.T) {
	var body []byte
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"rule-1","name":"block-scanners","filter_expr":"true","action":"deny","domain_selection_type":"include","domain_ids":["domain-uuid-1"]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := createArvanCloudAccountFirewallRuleTool(app.NewCreateArvanCloudAccountFirewallRule(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":               "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"name":                  "block-scanners",
		"filter_expr":           "true",
		"action":                "deny",
		"domain_selection_type": "include",
		"domain_ids":            []string{"domain-uuid-1"},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["domain_selection_type"] != "include" {
		t.Errorf(`request body["domain_selection_type"] = %v, want "include"`, sent["domain_selection_type"])
	}

	out, ok := result.(map[string]any)
	if !ok || out["id"] != "rule-1" {
		t.Errorf("result = %#v, want id=rule-1", result)
	}
}

// TestAttachArvanCloudAccountFirewallDomainsTool pins the request path/body
// sent to account.firewall_rules.add_domains.
func TestAttachArvanCloudAccountFirewallDomainsTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":"rule-1","domain_ids":["domain-uuid-1"]}}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := attachArvanCloudAccountFirewallDomainsTool(app.NewAttachArvanCloudAccountFirewallDomains(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":    "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"id":         "rule-1",
		"domain_ids": []string{"domain-uuid-1"},
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/account/firewall-rules/rule-1/domains" {
		t.Errorf("request path = %q, want /account/firewall-rules/rule-1/domains", path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	ids, ok := sent["domain_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "domain-uuid-1" {
		t.Errorf("request body[\"domain_ids\"] = %#v, want [\"domain-uuid-1\"]", sent["domain_ids"])
	}
}

// TestReprioritizeArvanCloudFirewallRulesTool pins the request body sent to
// firewall.reprioritize.
func TestReprioritizeArvanCloudFirewallRulesTool(t *testing.T) {
	var body []byte
	var path string
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"Successfully changed the priority of the rule."}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := reprioritizeArvanCloudFirewallRulesTool(app.NewReprioritizeArvanCloudFirewallRules(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":       "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":        "example.com",
		"rule_id":       "rule-1",
		"after_rule_id": "rule-2",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler() error = %v", err)
	}
	if path != "/domains/example.com/firewall/actions/reprioritize" {
		t.Errorf("request path = %q, want /domains/example.com/firewall/actions/reprioritize", path)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decoding the request the adapter sent: %v", err)
	}
	if sent["rule_id"] != "rule-1" || sent["after_rule_id"] != "rule-2" {
		t.Errorf("request body = %+v, want rule_id=rule-1 after_rule_id=rule-2", sent)
	}

	out, ok := result.(map[string]any)
	if !ok || out["reprioritized"] != true {
		t.Errorf("result = %#v, want reprioritized=true", result)
	}
}

// TestReprioritizeArvanCloudFirewallRulesToolRejectsBothAfterAndBefore
// proves the tool rejects giving both after_rule_id and before_rule_id
// before the provider is ever called.
func TestReprioritizeArvanCloudFirewallRulesToolRejectsBothAfterAndBefore(t *testing.T) {
	called := false
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := reprioritizeArvanCloudFirewallRulesTool(app.NewReprioritizeArvanCloudFirewallRules(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{
		"api_key":        "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d",
		"domain":         "example.com",
		"rule_id":        "rule-1",
		"after_rule_id":  "rule-2",
		"before_rule_id": "rule-3",
	})
	if err != nil {
		t.Fatalf("marshaling tool args: %v", err)
	}

	if _, err := tool.Handler(context.Background(), args); err == nil {
		t.Fatal("Handler() error = nil, want a validation error for giving both after_rule_id and before_rule_id")
	}
	if called {
		t.Error("provider was called despite giving both after_rule_id and before_rule_id")
	}
}

// TestListArvanCloudAccountFirewallValidDomainsTool is an end-to-end round
// trip for the unscoped account-level valid-domains listing.
func TestListArvanCloudAccountFirewallValidDomainsTool(t *testing.T) {
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"domain-uuid-1","name":"example.com"}]}`))
	}))
	defer providerSrv.Close()

	provider := newArvanCloudFirewallProvider(t, providerSrv)
	tool := listArvanCloudAccountFirewallValidDomainsTool(app.NewListArvanCloudAccountFirewallValidDomains(inlineQueue{}, provider))

	args, err := json.Marshal(map[string]any{"api_key": "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"})
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
	domains, ok := out["domains"].([]map[string]any)
	if !ok || len(domains) != 1 || domains[0]["domain"] != "example.com" {
		t.Errorf("result[domains] = %#v, want one entry with domain=example.com", out["domains"])
	}
}
